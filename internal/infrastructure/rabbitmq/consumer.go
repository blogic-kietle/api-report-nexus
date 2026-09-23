package rabbitmq

import (
	"context"
	"errors"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// MailQueue is where the scheduler drops SendReportMailRequest messages; it declares the topology itself.
const MailQueue = "report.mail.inbound"

// Reply answers a consumed message on the queue it named in reply_to, echoing the ids the scheduler correlates on.
func (p *Publisher) Reply(ctx context.Context, d amqp.Delivery, body []byte) error {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	return p.publish(ctx, "", d.ReplyTo, amqp.Publishing{
		ContentType:   "application/x-protobuf",
		DeliveryMode:  amqp.Persistent,
		MessageId:     d.MessageId,
		CorrelationId: d.CorrelationId,
		Body:          body,
	})
}

// Consume runs on its own connection, one message at a time, reconnecting until ctx ends; handle must ack or nack.
func (p *Publisher) Consume(ctx context.Context, queue string, handle func(amqp.Delivery)) {
	for ctx.Err() == nil {
		if err := p.consume(ctx, queue, handle); err != nil && p.cfg.Log != nil {
			p.cfg.Log.Error("rabbitmq consumer stopped, reconnecting", "queue", queue, "err", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}
}

func (p *Publisher) consume(ctx context.Context, queue string, handle func(amqp.Delivery)) error {
	conn, err := amqp.DialConfig(p.url, p.amqp)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	if err := ch.Qos(1, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	if p.cfg.Log != nil {
		p.cfg.Log.Info("rabbitmq consuming", "queue", queue)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("rabbitmq: channel closed")
			}
			handle(d)
		}
	}
}
