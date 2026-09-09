package notifications

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"notifications/internal/delivery"
)

type repositorySpy struct {
	calls             []string
	providerMessageID string
	failureOutcome    string
	failureReason     string
}

func (*repositorySpy) ClaimBatch(
	context.Context,
	string,
	int,
	time.Duration,
) ([]Notification, error) {
	return nil, nil
}

func (r *repositorySpy) MarkSent(
	_ context.Context,
	_ Notification,
	_ string,
	providerMessageID string,
	_ time.Time,
	_ time.Time,
) error {
	r.calls = append(r.calls, "mark_sent")
	r.providerMessageID = providerMessageID
	return nil
}

func (r *repositorySpy) MarkFailed(
	_ context.Context,
	_ Notification,
	_ string,
	outcome string,
	_ time.Time,
	_ string,
	reason string,
	_ time.Time,
	_ time.Time,
) error {
	r.calls = append(r.calls, "mark_failed")
	r.failureOutcome = outcome
	r.failureReason = reason
	return nil
}

type senderStub struct {
	calls    *[]string
	messages []delivery.Message
	resultID string
	err      error
}

func (*senderStub) Channel() string {
	return "email"
}

func (s *senderStub) Send(_ context.Context, message delivery.Message) (string, error) {
	*s.calls = append(*s.calls, "send")
	s.messages = append(s.messages, message)
	return s.resultID, s.err
}

func TestWorkerMarksSentOnlyAfterProviderAccepts(t *testing.T) {
	repository := &repositorySpy{}
	sender := &senderStub{
		calls:    &repository.calls,
		resultID: "provider-message-42",
	}
	worker := newTestWorker(t, repository, sender)
	job := testNotification()

	if err := worker.processNotification(context.Background(), job); err != nil {
		t.Fatalf("process notification: %v", err)
	}

	assertCalls(t, repository.calls, []string{"send", "mark_sent"})
	if repository.providerMessageID != sender.resultID {
		t.Fatalf(
			"provider message ID = %q, want %q",
			repository.providerMessageID,
			sender.resultID,
		)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("sent messages = %d, want 1", len(sender.messages))
	}
	if sender.messages[0].IdempotencyKey != job.ID {
		t.Fatalf(
			"idempotency key = %q, want notification ID %q",
			sender.messages[0].IdempotencyKey,
			job.ID,
		)
	}
}

func TestWorkerRetriesWithTheSameIdempotencyKey(t *testing.T) {
	repository := &repositorySpy{}
	sender := &senderStub{
		calls: &repository.calls,
		err:   errors.New("provider connection lost"),
	}
	worker := newTestWorker(t, repository, sender)
	job := testNotification()

	if err := worker.processNotification(context.Background(), job); err != nil {
		t.Fatalf("first attempt: %v", err)
	}
	job.Attempts++
	if err := worker.processNotification(context.Background(), job); err != nil {
		t.Fatalf("second attempt: %v", err)
	}

	assertCalls(
		t,
		repository.calls,
		[]string{"send", "mark_failed", "send", "mark_failed"},
	)
	if len(sender.messages) != 2 {
		t.Fatalf("sent messages = %d, want 2", len(sender.messages))
	}
	for attempt, message := range sender.messages {
		if message.IdempotencyKey != job.ID {
			t.Fatalf(
				"attempt %d idempotency key = %q, want %q",
				attempt+1,
				message.IdempotencyKey,
				job.ID,
			)
		}
	}
	if repository.failureOutcome != StatusRetry {
		t.Fatalf("failure outcome = %q, want %q", repository.failureOutcome, StatusRetry)
	}
	if repository.failureReason != sender.err.Error() {
		t.Fatalf("failure reason = %q, want %q", repository.failureReason, sender.err)
	}
}

func newTestWorker(
	t *testing.T,
	repository Repository,
	sender delivery.Sender,
) *Worker {
	t.Helper()

	worker, err := NewWorker(
		repository,
		[]delivery.Sender{sender},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		"worker-1",
		1,
		time.Second,
		time.Minute,
		5,
		time.Second,
	)
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}

	return worker
}

func testNotification() Notification {
	return Notification{
		ID:        "019946b9-b719-7aed-81b1-dcb23b2e57ea",
		Channel:   "email",
		Recipient: "user@example.com",
		Template:  "welcome_v1",
		Payload:   []byte(`{"name":"Test"}`),
		Attempts:  1,
	}
}

func assertCalls(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("calls = %v, want %v", got, want)
		}
	}
}
