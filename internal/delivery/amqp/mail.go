package amqpdelivery

import (
	"context"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"

	mailv1 "api-report-nexus/internal/gen/blogic/report/mail/v1"
	"api-report-nexus/internal/infrastructure/rabbitmq"
	"api-report-nexus/internal/usecase/scheduledmail"
)

// Mail answers each request twice, as the scheduler expects: ACCEPTED once decoded, then the outcome.
func Mail(ctx context.Context, bus *rabbitmq.Publisher, svc scheduledmail.Service, log *slog.Logger) func(amqp.Delivery) {
	return func(d amqp.Delivery) {
		req, err := scheduledmail.Decode(d.Body)
		if err != nil {
			log.Error("scheduled mail: undecodable message dropped", "err", err, "message_id", d.MessageId)
			_ = d.Nack(false, false)
			return
		}
		reply := func(r *mailv1.SendReportMailResult) {
			body, _ := proto.Marshal(r)
			if err := bus.Reply(ctx, d, body); err != nil {
				log.Error("scheduled mail: reply not published", "err", err, "message_id", d.MessageId, "status", r.Status.String())
			}
		}
		reply(svc.Accepted(req.MessageId))
		_ = d.Ack(false)
		res := svc.Run(ctx, req)
		log.Info("scheduled mail", "message_id", req.MessageId, "task_id", req.TaskId, "store", req.GetMeta().GetStoreId(),
			"reports", len(req.Reports), "status", res.Status.String(), "error", res.ErrorMessage)
		reply(res)
	}
}
