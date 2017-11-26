package lua

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	lua "github.com/Shopify/go-lua"
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

const (
	kafkaDefaultRequestTimeout = 5000
	kafkaDefaultMessageTimeout = kafkaDefaultRequestTimeout * 2
)

func getArgKafkaOffset(l *lua.State, i int, fn string) kafka.Offset {
	userData := l.ToUserData(i)
	offset, ok := userData.(kafka.Offset)
	if !ok && userData != nil {
		panic(fmt.Errorf(
			"%d argument must be a pointer to an offset in call to builtin '%s' function: found %s",
			i, fn, l.TypeOf(i)))
	}
	return offset
}

func getArgKafkaMessage(l *lua.State, i int, fn string) *kafka.Message {
	msg, ok := l.ToUserData(i).(*kafka.Message)
	if !ok {
		panic(fmt.Errorf(
			"%d argument must be a pointer to an kafka message in call to builtin '%s' function: found %s",
			i, fn, l.TypeOf(i)))
	}
	return msg
}

// luaKafkaMessage will create a kafka message with the given properties.
// lua signature is function kafka_message(key, value, topic, partition, offsetptr) msgptr
// If partition is set to -1, any partition is used. A new offset can be created by using
// kafka_offset builtin.
func luaKafkaMessage(l *lua.State) int {
	key := getArgString(l, 1, luaNameKafkaMessageFn)
	value := getArgString(l, 2, luaNameKafkaMessageFn)
	topic := getArgString(l, 3, luaNameKafkaMessageFn)
	partition := int32(getArgInt(l, 4, luaNameKafkaMessageFn))
	offset := getArgKafkaOffset(l, 5, luaNameKafkaMessageFn)

	var keyB []byte
	var valueB []byte

	if key == "" {
		keyB = nil
	} else {
		keyB = []byte(key)
	}

	if value == "" {
		valueB = nil
	} else {
		valueB = []byte(value)
	}

	message := kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: partition,
			Offset:    offset,
		},
		Key:   keyB,
		Value: valueB,
	}

	l.PushUserData(&message)

	return 1
}

// luaKAfkaProduce will produce the given message asynchronously. Delivery reports are
// dispatched via message report lua callback.
// lua signature is function kafka_produce (msgptr)
func luaKafkaProduce(l *lua.State) int {
	message := getArgKafkaMessage(l, 1, luaNameKafkaProduceFn)
	sandbox := getStateSandbox(l, 2)

	if sandbox.kafka == nil {
		if err := sandbox.initKafka(); err != nil {
			panic(fmt.Errorf("error initializing kafka resources: %v", err))
		}
	}
	channel := sandbox.kafka.ProduceChannel()
	sandbox.luaLock.Unlock()
	defer sandbox.luaLock.Lock()
	channel <- message
	return 0
}

// luaKafkaMessage will create a new kafka message.
// lua signature is function kafka_offset(name) offsetptr, err
func luaKafkaOffset(l *lua.State) int {
	name := getArgString(l, 1, luaNameKafkaOffsetFn)
	offset, err := kafka.NewOffset(name)
	if err != nil {
		l.PushNil()
		l.PushString(fmt.Sprintf("%s", err))
	} else {
		l.PushUserData(offset)
		l.PushNil()
	}
	return 2
}

// expected values by ConfigMap.SetKey are: string,bool,int,ConfigMap
// lua's builtins are represented by float64, float32, int64, int32
func normalizeKafkaValue(value interface{}) interface{} {
	switch value.(type) {
	case float64:
		return int(value.(float64))
	case float32:
		return int(value.(float32))
	case int64:
		return int(value.(int64))
	case int32:
		return int(value.(int32))
		// panic will occur later if type is not valid
	default:
		return value
	}
}

func getKafkaConfigError(key string, value interface{}) error {
	return fmt.Errorf("error setting kafka config key '%s' to value %v of type %v (expected string,bool,int,ConfigMap)",
		key, value, reflect.TypeOf(value))
}

func (l *Sandbox) setKafkaConfig(key string, value interface{}) bool {
	if !strings.HasPrefix(key, "kafka.") {
		return false
	}

	key = strings.TrimLeft(key, "kafka.")
	value = normalizeKafkaValue(value)

	if err := l.kafkaConfig.SetKey(key, value); err != nil {
		panic(getKafkaConfigError(key, value))
	}
	// TODO tear down and re-initialize
	// if l.kafka != nil {
	// 	l.kafka.Close()
	// }
	return true
}

func (l *Sandbox) callOnKafkaReport(m *kafka.Message) {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()
	l.state.Global(luaNameOnKafkaReportFn)
	if !l.state.IsFunction(-1) {
		l.state.Pop(-1)
		return
	}

	l.state.PushUserData(m)

	if err := m.TopicPartition.Error; err == nil {
		l.state.PushNil()
	} else {
		topic := *m.TopicPartition.Topic
		partition := int(m.TopicPartition.Partition)
		offset := int(m.TopicPartition.Offset)
		l.state.PushString(fmt.Sprintf("error when producing message to topic '%s' at partition %d with offset %d: %s",
			topic, partition, offset, err))
	}

	l.state.Call(2, 0)
}

func (l *Sandbox) pollKafkaEvents() {
	for ev := range l.kafka.Events() {
		switch ev.(type) {
		case *kafka.Message:
			l.callOnKafkaReport(ev.(*kafka.Message))
		case *kafka.Error:
			fmt.Fprintf(os.Stderr, "kafka error: %+v", ev)
		default:
			panic(fmt.Errorf("unexpected kafka event: %s", ev))
		}
	}
}

// depends on client configuration, we need to make sure that all message reports have
// been delivered and client has received all errors
func (l *Sandbox) getFlushTimeout() int {
	v, err := l.kafkaConfig.Get("message.timeout.ms", kafkaDefaultMessageTimeout)
	if err != nil {
		panic(err)
	}
	return v.(int) * 2
}

// Sets the following kafka config properties:
//
//   kafka.go.batch.producer (bool, false) - Enable batch producer (experimental for increased performance).
//                                     These batches do not relate to Kafka message batches in any way.
//   kafka.go.delivery.reports (bool, true) - Forward per-message delivery reports to the
//                                      Events() channel.
//   kafka.go.events.channel.size (int, 100) - Events() channel size
//   kafka.go.produce.channel.size (int, 100) - ProduceChannel() buffer size (in number of messages)
func setSaneKafkaDefaults(config *kafka.ConfigMap) {
	config.SetKey("go.batch.producer", false)
	config.SetKey("go.delivery.reports", true)
	config.SetKey("go.events.channel.size", 100)
	config.SetKey("go.produce.channel.size", 100)
	config.SetKey("queue.buffering.max.messages", 10000)
	config.SetKey("socket.timeout.ms", 5000)

	config.SetKey("default.topic.config", kafka.ConfigMap{
		"request.required.acks": 1,
		// once message is produced, ack timeout
		"request.timeout.ms": kafkaDefaultRequestTimeout,
		// locally limits the time a produced message waits for successful delivery
		"message.timeout.ms": kafkaDefaultMessageTimeout,
	})
}
