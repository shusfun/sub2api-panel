package panelupdate

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"sub2api-panel/internal/platform"
)

const (
	queued       = "queued"
	download     = "download"
	maintenance  = "maintenance"
	backup       = "backup"
	migrate      = "migrate"
	switching    = "switching"
	starting     = "starting"
	recovering   = "recovering"
	succeeded    = "succeeded"
	failed       = "failed"
	rolledBack   = "rolled_back"
	intervention = "intervention"
)

type persistedJob struct {
	Job
	Actor          int64  `json:"actor"`
	Trace          string `json:"trace"`
	Parent         string `json:"parent"`
	Migration      string `json:"migration"`
	BinaryRollback bool   `json:"binary_rollback"`
	Previous       string `json:"previous"`
	Proposed       string `json:"proposed"`
}

type host interface {
	current(context.Context) (string, string, error)
	prepare(context.Context, string, string) error
	setMaintenance(bool) error
	stop(context.Context) error
	backup(context.Context, string) error
	migrate(context.Context, string) error
	selectBinary(string) error
	start(context.Context, string) error
	ready(context.Context, string) error
}

type runner struct {
	dir       string
	channel   string
	source    releaseSource
	host      host
	mu        sync.Mutex
	job       *persistedJob
	checkGate chan struct{}
}

func newRunner(dir, channel string, source releaseSource, h host) (*runner, error) {
	r := &runner{dir: dir, channel: channel, source: source, host: h, checkGate: make(chan struct{}, 1)}
	file, err := os.Open(r.statePath())
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var job persistedJob
	if err := readStrict(file, 16<<10, &job); err != nil {
		return nil, err
	}
	if !platform.TraceHex.MatchString(job.ID) || !ValidVersion(job.From) || !ValidVersion(job.To) {
		return nil, errors.New("invalid saved update job")
	}
	r.job = &job
	return r, nil
}

func (r *runner) statePath() string { return filepath.Join(r.dir, "latest-job.json") }

func (r *runner) save(job persistedJob) error {
	job.UpdatedAt = time.Now().UTC()
	if err := writeState(r.statePath(), job); err != nil {
		return err
	}
	r.job = &job
	return nil
}

func writeState(path string, value any) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".state-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	writeErr := json.NewEncoder(file).Encode(value)
	if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	return syncDir(dir)
}

func busy(job *persistedJob) bool {
	if job == nil {
		return false
	}
	switch job.Phase {
	case succeeded, failed, rolledBack:
		return false
	default:
		return true
	}
}

func (r *runner) serve(l net.Listener) error {
	if busy(r.job) && r.job.Phase != intervention {
		job := *r.job
		go r.resume(job)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", r.status)
	mux.HandleFunc("GET /release", r.check)
	mux.HandleFunc("POST /jobs", r.submit)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 45 * time.Second}
	return server.Serve(l)
}

func (r *runner) status(w http.ResponseWriter, _ *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := Status{Configured: true}
	if r.job != nil {
		job := r.job.Job
		out.Job = &job
	}
	platform.WriteJSON(w, http.StatusOK, out)
}

func (r *runner) check(w http.ResponseWriter, req *http.Request) {
	req, end := platform.TraceRequest(req, w, "检查面板更新")
	defer end()
	select {
	case r.checkGate <- struct{}{}:
		defer func() { <-r.checkGate }()
	default:
		w.WriteHeader(http.StatusConflict)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 38*time.Second)
	defer cancel()
	current, _, err := r.host.current(ctx)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	result, err := r.source.find(ctx, r.dir, current, r.channel, "")
	if err != nil {
		logFailure(ctx, "check", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	var out Check
	if result != nil {
		out.Release = &result.info
	}
	platform.WriteJSON(w, http.StatusOK, out)
}

func (r *runner) submit(w http.ResponseWriter, req *http.Request) {
	parent := platform.SpanHex.FindStringSubmatch(req.Header.Get("traceparent"))
	req, end := platform.TraceRequest(req, w, "接收面板更新任务")
	defer end()
	var input Request
	if readStrict(req.Body, 1024, &input) != nil || input.Actor <= 0 || !ValidVersion(input.Version) || len(input.Version) > 40 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if busy(r.job) {
		w.WriteHeader(http.StatusConflict)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	current, previous, err := r.host.current(ctx)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if Compare(input.Version, current) <= 0 {
		w.WriteHeader(http.StatusConflict)
		return
	}
	meta := platform.Meta(req.Context())
	if len(parent) == 3 && parent[1] == meta.Trace {
		meta.Span = parent[2]
	}
	job := persistedJob{Job: Job{ID: platform.RandomHex(16), From: current, To: input.Version, Phase: queued},
		Actor: input.Actor, Trace: meta.Trace, Parent: meta.Span, Previous: previous}
	if err := r.save(job); err != nil {
		logFailure(req.Context(), "accept", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	platform.WriteJSON(w, http.StatusAccepted, r.job.Job)
	go r.run(job)
}

func (r *runner) stage(ctx context.Context, job *persistedJob, phase string, run func(context.Context) error) error {
	ctx, end := platform.StartFlow(ctx, "执行面板更新阶段", "job_id", job.ID, "phase", phase)
	defer end()
	if err := r.transition(job, phase, ""); err != nil {
		return err
	}
	return run(ctx)
}

func (r *runner) transition(job *persistedJob, phase, failure string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	next := *job
	next.Phase, next.Failure = phase, failure
	if err := r.save(next); err != nil {
		return err
	}
	*job = next
	return nil
}

func (r *runner) finish(ctx context.Context, job *persistedJob, phase, failure string) {
	if err := r.transition(job, phase, failure); err != nil {
		logFailure(ctx, "persist_result", err)
	}
	platform.UpdateLog(ctx, "面板更新结果", "job_id", job.ID, "phase", job.Phase, "failure", job.Failure)
}

func jobContext(job persistedJob) (context.Context, func()) {
	ctx := platform.WithMeta(context.Background(), platform.CallMeta{Trace: job.Trace, Span: job.Parent})
	return platform.StartFlow(ctx, "执行面板更新", "job_id", job.ID, "actor_id", job.Actor, "from", job.From, "to", job.To)
}

func (r *runner) run(job persistedJob) {
	ctx, end := jobContext(job)
	defer end()
	if err := r.stage(ctx, &job, download, func(ctx context.Context) error { return r.prepare(ctx, &job) }); err != nil {
		logFailure(ctx, download, err)
		r.finish(ctx, &job, failed, download)
		return
	}
	err := r.install(ctx, &job)
	if err != nil {
		logFailure(ctx, job.Phase, err)
		r.recover(ctx, &job, job.Phase)
	}
}

func (r *runner) prepare(ctx context.Context, job *persistedJob) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()
	current, previous, err := r.host.current(ctx)
	if err != nil {
		return err
	}
	if current != job.From || previous != job.Previous {
		return errors.New("installed version changed")
	}
	offer, err := r.source.find(ctx, r.dir, job.From, r.channel, job.To)
	if err != nil {
		return err
	}
	if offer == nil || offer.info.Version != job.To || offer.info.Availability != "ready" {
		return errors.New("requested release is no longer eligible")
	}
	work, err := os.MkdirTemp(r.dir, "update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err := r.source.prepare(ctx, work, offer); err != nil {
		return err
	}
	if err := r.host.prepare(ctx, job.To, work); err != nil {
		return err
	}
	job.Proposed = job.To
	job.Migration = offer.manifest.Migration
	job.BinaryRollback = offer.manifest.BinaryRollback
	return nil
}

func (r *runner) install(ctx context.Context, job *persistedJob) error {
	if err := r.stage(ctx, job, maintenance, func(ctx context.Context) error {
		if err := r.host.setMaintenance(true); err != nil {
			return err
		}
		return r.host.stop(ctx)
	}); err != nil {
		return err
	}
	if err := r.stage(ctx, job, backup, func(ctx context.Context) error { return r.host.backup(ctx, job.ID) }); err != nil {
		return err
	}
	if job.Migration == "apply" {
		if err := r.stage(ctx, job, migrate, func(ctx context.Context) error { return r.host.migrate(ctx, job.Proposed) }); err != nil {
			return err
		}
	}
	return r.activate(ctx, job)
}

func (r *runner) activate(ctx context.Context, job *persistedJob) error {
	if err := r.stage(ctx, job, switching, func(context.Context) error { return r.host.selectBinary(job.Proposed) }); err != nil {
		return err
	}
	if err := r.stage(ctx, job, starting, func(ctx context.Context) error { return r.host.start(ctx, job.To) }); err != nil {
		return err
	}
	if err := r.host.setMaintenance(false); err != nil {
		return err
	}
	r.finish(ctx, job, succeeded, "")
	return nil
}

func (r *runner) resume(job persistedJob) {
	ctx, end := jobContext(job)
	defer end()
	switch job.Phase {
	case queued, download:
		r.finish(ctx, &job, failed, "interrupted")
	case switching, starting:
		probe, cancel := context.WithTimeout(ctx, 4*time.Second)
		ready := r.host.ready(probe, job.To) == nil
		cancel()
		if ready {
			if err := r.host.setMaintenance(false); err == nil {
				r.finish(ctx, &job, succeeded, "")
				return
			}
		}
		if err := r.restartActivation(ctx, &job); err == nil {
			return
		} else {
			logFailure(ctx, job.Phase, err)
		}
		r.recover(ctx, &job, "interrupted")
	default:
		r.recover(ctx, &job, "interrupted")
	}
}

func (r *runner) restartActivation(ctx context.Context, job *persistedJob) error {
	if err := r.host.setMaintenance(true); err != nil {
		return err
	}
	if err := r.host.stop(ctx); err != nil {
		return err
	}
	return r.activate(ctx, job)
}

func (r *runner) recover(ctx context.Context, job *persistedJob, failure string) {
	phase := job.Phase
	if phase == queued || phase == download {
		r.finish(ctx, job, failed, failure)
		return
	}
	if phase == migrate || ((phase == switching || phase == starting) && !job.BinaryRollback && job.Migration != "none") {
		r.leaveMaintenance(ctx, job, failure)
		return
	}
	err := r.stage(ctx, job, recovering, func(ctx context.Context) error {
		if err := r.host.setMaintenance(true); err != nil {
			return err
		}
		if err := r.host.stop(ctx); err != nil {
			return err
		}
		if err := r.host.selectBinary(job.Previous); err != nil {
			return err
		}
		return r.host.start(ctx, job.From)
	})
	if err == nil {
		err = r.host.setMaintenance(false)
	}
	if err != nil {
		logFailure(ctx, recovering, err)
		r.leaveMaintenance(ctx, job, failure)
		return
	}
	r.finish(ctx, job, rolledBack, failure)
}

func (r *runner) leaveMaintenance(ctx context.Context, job *persistedJob, failure string) {
	if err := errors.Join(r.host.setMaintenance(true), r.host.stop(ctx)); err != nil {
		logFailure(ctx, intervention, err)
	}
	r.finish(ctx, job, intervention, failure)
}

func logFailure(ctx context.Context, phase string, err error) {
	platform.UpdateLog(ctx, "面板更新失败", "phase", phase, "error", err.Error())
}
