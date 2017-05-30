package kafka

import (
	"bufio"
	"bytes"
	"fmt"
	"hash/crc32"
	"reflect"
	"testing"
)

func TestMessageCRC32(t *testing.T) {
	t.Parallel()

	m := message{
		MagicByte: 1,
		Timestamp: 42,
		Key:       nil,
		Value:     []byte("Hello World!"),
	}

	b := &bytes.Buffer{}
	w := bufio.NewWriter(b)
	write(w, m)
	w.Flush()

	h := crc32.NewIEEE()
	h.Write(b.Bytes()[4:])

	sum1 := h.Sum32()
	sum2 := uint32(m.crc32())

	if sum1 != sum2 {
		t.Error("bad CRC32:")
		t.Logf("expected: %d", sum1)
		t.Logf("found:    %d", sum2)
	}
}

func TestProtocol(t *testing.T) {
	t.Parallel()

	tests := []interface{}{
		requestHeader{
			Size:          26,
			ApiKey:        int16(offsetCommitRequest),
			ApiVersion:    int16(v2),
			CorrelationID: 42,
			ClientID:      "Hello World!",
		},

		message{
			MagicByte: 1,
			Timestamp: 42,
			Key:       nil,
			Value:     []byte("Hello World!"),
		},

		topicMetadataRequestV0{"A", "B", "C"},

		metadataResponseV0{
			Brokers: []brokerMetadataV0{
				{NodeID: 1, Host: "localhost", Port: 9001},
				{NodeID: 2, Host: "localhost", Port: 9002},
			},
			Topics: []topicMetadataV0{
				{TopicErrorCode: 0, Partitions: []partitionMetadataV0{{
					PartitionErrorCode: 0,
					PartitionID:        1,
					Leader:             2,
					Replicas:           []int32{1},
					Isr:                []int32{1},
				}}},
			},
		},

		listOffsetRequestV1{
			ReplicaID: 1,
			Topics: []listOffsetRequestTopicV1{
				{TopicName: "A", Partitions: []listOffsetRequestPartitionV1{
					{Partition: 0, Time: -1},
					{Partition: 1, Time: -1},
					{Partition: 2, Time: -1},
				}},
				{TopicName: "B", Partitions: []listOffsetRequestPartitionV1{
					{Partition: 0, Time: -2},
				}},
				{TopicName: "C", Partitions: []listOffsetRequestPartitionV1{
					{Partition: 0, Time: 42},
				}},
			},
		},

		[]listOffsetResponseV1{
			{TopicName: "A", PartitionOffsets: []partitionOffsetV1{
				{Partition: 0, Timestamp: 42, Offset: 1},
			}},
			{TopicName: "B", PartitionOffsets: []partitionOffsetV1{
				{Partition: 0, Timestamp: 43, Offset: 10},
				{Partition: 1, Timestamp: 44, Offset: 100},
			}},
		},
	}

	for _, test := range tests {
		value := test
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) {
			t.Parallel()

			b := &bytes.Buffer{}
			r := bufio.NewReader(b)
			w := bufio.NewWriter(b)

			if err := write(w, value); err != nil {
				t.Fatal(err)
			}
			if err := w.Flush(); err != nil {
				t.Fatal(err)
			}

			if size := int(sizeof(value)); size != b.Len() {
				t.Error("invalid size:", size, "!=", b.Len())
			}

			v := reflect.New(reflect.TypeOf(value))
			n := b.Len()

			n, err := read(r, n, v.Interface())
			if err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("%d unread bytes", n)
			}

			if !reflect.DeepEqual(value, v.Elem().Interface()) {
				t.Error("values don't match:")
				t.Logf("expected: %#v", value)
				t.Logf("found:    %#v", v.Elem().Interface())
			}
		})
	}
}
