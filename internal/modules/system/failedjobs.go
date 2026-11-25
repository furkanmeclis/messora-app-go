package system

import (
	"cmp"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hibiken/asynq"

	"github.com/furkanmeclis/messora-app-go/internal/platform/queue"
	"github.com/furkanmeclis/messora-app-go/internal/shared/apperr"
)

// maxScan bounds how many of the newest failed jobs of one queue are loaded
// for a listing (asynq itself keeps at most 10,000 per queue for 90 days).
const maxScan = 1000

// FailedJob is a dead-lettered task as shown to operators: identifiers only,
// never the payload.
type FailedJob struct {
	ID        string
	Queue     string
	Type      string
	LastError string
	FailedAt  time.Time
	Retried   int
	MaxRetry  int
	Refs      map[string]string
}

// FailedJobs lists, retries and deletes archived asynq tasks.
type FailedJobs struct {
	insp   *asynq.Inspector
	queues []string
}

// NewFailedJobs uses insp for every queue in queue.AllQueues.
func NewFailedJobs(insp *asynq.Inspector) *FailedJobs {
	return &FailedJobs{insp: insp, queues: queue.AllQueues}
}

var errJobNotFound = apperr.NotFound("Failed job not found.")

func (f *FailedJobs) known(q string) bool { return slices.Contains(f.queues, q) }

// existing filters queues down to those asynq knows (a queue exists once a
// task was enqueued there; GetQueueInfo fails for the others).
func (f *FailedJobs) existing(queues []string) ([]string, error) {
	all, err := f.insp.Queues()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(queues))
	for _, q := range queues {
		if slices.Contains(all, q) {
			out = append(out, q)
		}
	}
	return out, nil
}

// Count returns the number of failed jobs per existing queue.
func (f *FailedJobs) Count() (map[string]int, error) {
	queues, err := f.existing(f.queues)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, q := range queues {
		info, err := f.insp.GetQueueInfo(q)
		if err != nil {
			return nil, err
		}
		out[q] = info.Archived
	}
	return out, nil
}

// List returns a page (1-based) of failed jobs, newest first, from one queue
// or all of them, and the total number of failed jobs.
func (f *FailedJobs) List(queueName string, page, limit int) ([]FailedJob, int64, error) {
	queues := f.queues
	if queueName != "" {
		if !f.known(queueName) {
			return nil, 0, apperr.Validation("queue", "Unknown queue.")
		}
		queues = []string{queueName}
	}
	queues, err := f.existing(queues)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	var all []*asynq.TaskInfo
	for _, q := range queues {
		info, err := f.insp.GetQueueInfo(q)
		if err != nil {
			return nil, 0, err
		}
		if info.Archived == 0 {
			continue
		}
		total += int64(info.Archived)
		tasks, err := f.newest(q, info.Archived)
		if err != nil {
			return nil, 0, err
		}
		all = append(all, tasks...)
	}
	slices.SortFunc(all, func(a, b *asynq.TaskInfo) int {
		if c := b.LastFailedAt.Compare(a.LastFailedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	start := (page - 1) * limit
	if start >= len(all) {
		return []FailedJob{}, total, nil
	}
	end := min(start+limit, len(all))
	out := make([]FailedJob, 0, end-start)
	for _, t := range all[start:end] {
		out = append(out, toFailedJob(t))
	}
	return out, total, nil
}

// newest loads up to about maxScan of the most recently archived tasks of a
// queue (asynq lists archived tasks oldest first, in fixed-size pages).
func (f *FailedJobs) newest(q string, n int) ([]*asynq.TaskInfo, error) {
	if n <= maxScan {
		return f.insp.ListArchivedTasks(q, asynq.PageSize(n), asynq.Page(1))
	}
	last := (n-1)/maxScan + 1
	tasks, err := f.insp.ListArchivedTasks(q, asynq.PageSize(maxScan), asynq.Page(last))
	if err != nil {
		return nil, err
	}
	if len(tasks) < maxScan {
		prev, err := f.insp.ListArchivedTasks(q, asynq.PageSize(maxScan), asynq.Page(last-1))
		if err != nil {
			return nil, err
		}
		tasks = append(prev, tasks...)
	}
	return tasks, nil
}

// get returns a task only while it is archived.
func (f *FailedJobs) get(q, id string) (*asynq.TaskInfo, error) {
	if !f.known(q) {
		return nil, errJobNotFound
	}
	t, err := f.insp.GetTaskInfo(q, id)
	if errors.Is(err, asynq.ErrQueueNotFound) || errors.Is(err, asynq.ErrTaskNotFound) {
		return nil, errJobNotFound
	}
	if err != nil {
		return nil, err
	}
	if t.State != asynq.TaskStateArchived {
		return nil, errJobNotFound
	}
	return t, nil
}

// Retry moves a failed job back to the pending state.
func (f *FailedJobs) Retry(q, id string) (FailedJob, error) {
	t, err := f.get(q, id)
	if err != nil {
		return FailedJob{}, err
	}
	if err := f.insp.RunTask(q, id); err != nil {
		if errors.Is(err, asynq.ErrTaskNotFound) {
			return FailedJob{}, errJobNotFound
		}
		return FailedJob{}, err
	}
	return toFailedJob(t), nil
}

// Delete discards a failed job.
func (f *FailedJobs) Delete(q, id string) (FailedJob, error) {
	t, err := f.get(q, id)
	if err != nil {
		return FailedJob{}, err
	}
	if err := f.insp.DeleteTask(q, id); err != nil {
		if errors.Is(err, asynq.ErrTaskNotFound) {
			return FailedJob{}, errJobNotFound
		}
		return FailedJob{}, err
	}
	return toFailedJob(t), nil
}

func toFailedJob(t *asynq.TaskInfo) FailedJob {
	return FailedJob{ID: t.ID, Queue: t.Queue, Type: t.Type, LastError: redactError(t.LastErr),
		FailedAt: t.LastFailedAt, Retried: t.Retried, MaxRetry: t.MaxRetry, Refs: payloadRefs(t.Payload)}
}

// payloadRefs keeps only identifier fields of a JSON object payload: keys
// ending in "_id" (or "id") with short scalar values. Everything else —
// message bodies, contact data, anything secret — is dropped.
func payloadRefs(payload []byte) map[string]string {
	out := map[string]string{}
	var m map[string]any
	if json.Unmarshal(payload, &m) != nil {
		return out
	}
	for k, v := range m {
		if k != "id" && !strings.HasSuffix(k, "_id") {
			continue
		}
		switch x := v.(type) {
		case string:
			if x != "" && len(x) <= 100 {
				out[k] = x
			}
		case float64:
			out[k] = strconv.FormatFloat(x, 'f', -1, 64)
		}
	}
	return out
}

var (
	secretParam = regexp.MustCompile(`(?i)\b([a-z_]*(?:token|secret|password|passwd|api_?key|signature)[a-z_]*)(["']?\s*[:=]\s*["']?)[^\s&"',}]+`)
	bearer      = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[a-z0-9._~+/=-]+`)
	userinfo    = regexp.MustCompile(`(?i)(\b[a-z][a-z0-9+.-]*://)[^/\s:@]+:[^/\s@]+@`)
)

// redactError masks credentials an error message may quote (URL query
// parameters, JSON fields, auth headers, URL user info) and bounds its size.
func redactError(s string) string {
	s = secretParam.ReplaceAllString(s, "${1}${2}[redacted]")
	s = bearer.ReplaceAllString(s, "${1} [redacted]")
	s = userinfo.ReplaceAllString(s, "${1}[redacted]@")
	const maxLen = 1000
	if len(s) > maxLen {
		cut := maxLen
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut] + "…"
	}
	return s
}
