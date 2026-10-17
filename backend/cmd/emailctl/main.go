// Command emailctl inspects and operates the email queues:
//
//	emailctl stats                show pending / dead-letter counts
//	emailctl list [n]             show up to n dead-lettered emails (codes are masked)
//	emailctl requeue [n|all]      move dead-lettered emails back for another round of retries
//	emailctl purge                delete every dead-lettered email
//
// Dead-lettered signup codes expire after ten minutes, so most old entries are
// only worth purging; requeue is for the case where the cause (an SMTP outage,
// a wrong credential) was fixed soon after.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/email"
	"backend/internal/queue"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "emailctl:", err)
		os.Exit(1)
	}
}

func usage() error {
	return fmt.Errorf("usage: emailctl stats | list [n] | requeue [n|all] | purge")
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	cfg, err := config.LoadRedisOnly()
	if err != nil {
		return err
	}
	rdb, err := db.ConnectRedis(cfg)
	if err != nil {
		return err
	}
	defer rdb.Close()
	b := queue.NewRedisListBroker(rdb, email.DLQ, queue.WithDLQMaxLen(email.DLQMaxLen))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	switch args[0] {
	case "stats":
		d, err := email.Snapshot(ctx, b)
		if err != nil {
			return err
		}
		fmt.Printf("pending:     %d  (%s)\ndead-letter: %d  (%s)\n", d.Pending, email.Topic, d.DeadLetter, email.DLQ)
	case "list":
		n := 20
		if len(args) > 1 {
			if n, err = strconv.Atoi(args[1]); err != nil || n < 1 {
				return usage()
			}
		}
		entries, err := b.Peek(ctx, email.DLQ, n)
		if err != nil {
			return err
		}
		for _, e := range entries {
			var m email.Message
			_ = json.Unmarshal(e.Payload, &m)
			if m.OTP != "" {
				m.OTP = "******" // never print live codes
			}
			fmt.Printf("%s  attempts=%d  to=%q type=%q otp=%s\n", e.ID, e.Attempts, m.To, m.Type, m.OTP)
		}
		fmt.Printf("%d shown\n", len(entries))
	case "requeue":
		n := 1000
		all := len(args) > 1 && args[1] == "all"
		if len(args) > 1 && !all {
			if n, err = strconv.Atoi(args[1]); err != nil || n < 1 {
				return usage()
			}
		}
		moved := 0
		for {
			m, err := b.Requeue(ctx, email.DLQ, email.Topic, n)
			moved += m
			if err != nil {
				fmt.Printf("requeued %d message(s)\n", moved)
				return err
			}
			// "all" keeps going until the DLQ is empty; a number stops after one batch.
			if !all || m == 0 {
				break
			}
		}
		fmt.Printf("requeued %d message(s)\n", moved)
		return nil
	case "purge":
		n, err := b.Purge(ctx, email.DLQ)
		fmt.Printf("purged %d message(s)\n", n)
		return err
	default:
		return usage()
	}
	return nil
}
