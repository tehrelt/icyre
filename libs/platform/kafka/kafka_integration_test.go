//go:build integration

package kafka

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Requires a running broker: KAFKA_BROKERS=localhost:9094 go test -tags integration ./kafka
func brokers(t *testing.T) []string {
	b := os.Getenv("KAFKA_BROKERS")
	if b == "" {
		t.Skip("KAFKA_BROKERS is not set")
	}
	return strings.Split(b, ",")
}

// createTopics creates topics explicitly, as deploy/kafka/create-topics.sh does.
func createTopics(t *testing.T, bs []string, topics ...string) {
	t.Helper()
	cl, err := kgo.NewClient(kgo.SeedBrokers(bs...))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	res, err := kadm.NewClient(cl).CreateTopics(context.Background(), 1, 1, nil, topics...)
	if err != nil {
		t.Fatal(err)
	}
	if err := res.Error(); err != nil {
		t.Fatal(err)
	}
}

func TestPublishConsumeAndShutdown(t *testing.T) {
	bs := brokers(t)
	topic := fmt.Sprintf("platform.it.%d", time.Now().UnixNano())
	createTopics(t, bs, topic)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	p, err := NewProducer(ProducerConfig{Brokers: bs, ClientID: "it"}, prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)

	if err := p.Publish(ctx, Message{Topic: topic, Key: []byte("k"), Value: []byte("hello")}); err != nil {
		t.Fatal(err)
	}

	got := make(chan Record, 1)
	c, err := NewConsumer(ConsumerConfig{Brokers: bs, Group: topic + ".group", Topics: []string{topic}},
		func(_ context.Context, r Record) error { got <- r; return nil }, log, NewConsumerMetrics(prometheus.NewRegistry()))
	if err != nil {
		t.Fatal(err)
	}

	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- c.Run(runCtx) }()

	select {
	case r := <-got:
		if string(r.Value) != "hello" || string(r.Key) != "k" {
			t.Fatalf("unexpected record %+v", r)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for record")
	}

	stop()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("consumer returned %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("consumer did not shut down")
	}
}

func TestFailedMessageGoesToDLQ(t *testing.T) {
	bs := brokers(t)
	topic := fmt.Sprintf("platform.it.dlq.%d", time.Now().UnixNano())
	createTopics(t, bs, topic, DLQTopic(topic))
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	p, err := NewProducer(ProducerConfig{Brokers: bs}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close(ctx)
	if err := p.Publish(ctx, Message{Topic: topic, Value: []byte("poison")}); err != nil {
		t.Fatal(err)
	}

	failing, err := NewConsumer(ConsumerConfig{Brokers: bs, Group: topic + ".g", Topics: []string{topic}, MaxRetries: 1, RetryBackoff: 10 * time.Millisecond, DLQ: true},
		func(context.Context, Record) error { return errors.New("cannot handle") }, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	fctx, fstop := context.WithCancel(ctx)
	go failing.Run(fctx)
	defer fstop()

	got := make(chan Record, 1)
	dlq, err := NewConsumer(ConsumerConfig{Brokers: bs, Group: topic + ".dlq.g", Topics: []string{DLQTopic(topic)}},
		func(_ context.Context, r Record) error { got <- r; return nil }, log, nil)
	if err != nil {
		t.Fatal(err)
	}
	dctx, dstop := context.WithCancel(ctx)
	go dlq.Run(dctx)
	defer dstop()

	select {
	case r := <-got:
		if string(r.Value) != "poison" || r.Headers["dlq.source.topic"] != topic {
			t.Fatalf("unexpected dlq record %+v", r)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for dlq record")
	}
}
