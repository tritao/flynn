package testutils

import (
	"errors"
	"sync"
	"time"

	"github.com/flynn/flynn/host/types"
	"github.com/flynn/flynn/pkg/cluster"
)

func NewFakeHostClient(hostID string) *FakeHostClient {
	return &FakeHostClient{
		hostID:  hostID,
		stopped: make(map[string]bool),
		attach:  make(map[string]attachFunc),
	}
}

type FakeHostClient struct {
	hostID    string
	stopped   map[string]bool
	attach    map[string]attachFunc
	cluster   *FakeCluster
	listeners []chan<- *host.Event
	listenMtx sync.RWMutex
}

func (c *FakeHostClient) ListJobs() (map[string]host.ActiveJob, error) { return nil, nil }
func (c *FakeHostClient) Close() error                                 { return nil }
func (c *FakeHostClient) Attach(req *host.AttachReq, wait bool) (cluster.ReadWriteCloser, func() error, error) {
	f, ok := c.attach[req.JobID]
	if !ok {
		f = c.attach["*"]
	}
	return f(req, wait)
}

func (c *FakeHostClient) GetJob(id string) (*host.ActiveJob, error) {
	hosts, err := c.cluster.ListHosts()
	if err != nil {
		return nil, err
	}

	for _, h := range hosts {
		for _, job := range h.Jobs {
			if job.ID == id {
				return &host.ActiveJob{Job: job}, nil
			}
		}
	}
	return nil, errors.New("job not found")
}

func (c *FakeHostClient) StreamEvents(id string, ch chan<- *host.Event) cluster.Stream {
	c.listenMtx.Lock()
	defer c.listenMtx.Unlock()
	c.listeners = append(c.listeners, ch)
	return &FakeHostEventStream{ch: ch}
}

func (c *FakeHostClient) StopJob(id string) error {
	c.stopped[id] = true
	c.cluster.RemoveJob(c.hostID, id, false)
	return nil
}

func (c *FakeHostClient) IsStopped(id string) bool {
	return c.stopped[id]
}

func (c *FakeHostClient) SetAttach(id string, rwc cluster.ReadWriteCloser) {
	c.attach[id] = func(*host.AttachReq, bool) (cluster.ReadWriteCloser, func() error, error) {
		return rwc, nil, nil
	}
}

func (c *FakeHostClient) SetAttachFunc(id string, f attachFunc) {
	c.attach[id] = f
}

func (c *FakeHostClient) SendEvent(event, id string) {
	c.listenMtx.RLock()
	defer c.listenMtx.RUnlock()
	job := &host.ActiveJob{Job: &host.Job{ID: id}}
	if event == "start" {
		job.StartedAt = time.Now().UTC()
	}
	e := &host.Event{Event: event, JobID: id, Job: job}
	for _, ch := range c.listeners {
		ch <- e
	}
}

type attachFunc func(req *host.AttachReq, wait bool) (cluster.ReadWriteCloser, func() error, error)

type FakeHostEventStream struct {
	ch chan<- *host.Event
}

func (h *FakeHostEventStream) Close() error {
	close(h.ch)
	return nil
}

func (h *FakeHostEventStream) Err() error {
	return nil
}
