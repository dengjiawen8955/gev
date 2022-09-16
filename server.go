package gev

import (
	"errors"
	"runtime"
	"time"

	"github.com/Allenxuxu/gev/eventloop"
	"github.com/Allenxuxu/gev/log"
	"github.com/Allenxuxu/toolkit/sync"
	"github.com/Allenxuxu/toolkit/sync/atomic"
	"github.com/RussellLuo/timingwheel"
	"golang.org/x/sys/unix"
)

// Handler Server 注册接口
type Handler interface {
	CallBack
	OnConnect(c *Connection)
}

// Server gev Server
type Server struct {
	listener  *listener              // 监听的主 Reactor
	workLoops []*eventloop.EventLoop // worker Reactor
	callback  Handler                // 用户定义的 Handler 函数

	timingWheel *timingwheel.TimingWheel // 类似 kafka 中的层级定时器
	opts        *Options                 // Options 配置
	running     atomic.Bool              // 状态是否为正在运行
}

// NewServer 创建 Server
// @param handler 用户定义的 Handler 处理逻辑
// @param opts 配置
func NewServer(handler Handler, opts ...Option) (server *Server, err error) {
	if handler == nil {
		return nil, errors.New("handler is nil")
	}
	options := newOptions(opts...)
	server = new(Server)
	server.callback = handler
	server.opts = options
	server.timingWheel = timingwheel.NewTimingWheel(server.opts.tick, server.opts.wheelSize)
	server.listener, err = newListener(server.opts.Network, server.opts.Address, options.ReusePort, server.handleNewConnection)
	if err != nil {
		return nil, err
	}

	if server.opts.NumLoops <= 0 {
		server.opts.NumLoops = runtime.NumCPU()
	}

	wloops := make([]*eventloop.EventLoop, server.opts.NumLoops)
	for i := 0; i < server.opts.NumLoops; i++ {
		l, err := eventloop.New()
		if err != nil {
			for j := 0; j < i; j++ {
				_ = wloops[j].Stop()
			}
			return nil, err
		}
		wloops[i] = l
	}
	server.workLoops = wloops

	return
}

// RunAfter 延时任务
func (s *Server) RunAfter(d time.Duration, f func()) *timingwheel.Timer {
	return s.timingWheel.AfterFunc(d, f)
}

// RunEvery 定时任务
func (s *Server) RunEvery(d time.Duration, f func()) *timingwheel.Timer {
	return s.timingWheel.ScheduleFunc(&everyScheduler{Interval: d}, f)
}

func (s *Server) handleNewConnection(fd int, sa unix.Sockaddr) {
	// 通过负载均衡找到一个 worker
	loop := s.opts.Strategy(s.workLoops)

	// 创建连接
	c := NewConnection(fd, loop, sa, s.opts.Protocol, s.timingWheel, s.opts.IdleTime, s.callback)

	// 添加一个事件, 将连接放到 worker 里面
	loop.QueueInLoop(func() {
		s.callback.OnConnect(c)
		if err := loop.AddSocketAndEnableRead(fd, c); err != nil {
			log.Error("[AddSocketAndEnableRead]", err)
		}
	})
}

// Start 启动 Server
// 在协程中启动主 Reactor 和 worker Reactor, 开始事件循环
func (s *Server) Start() {
	sw := sync.WaitGroupWrapper{}
	// 启动定时器
	s.timingWheel.Start()

	// 启动 worker
	length := len(s.workLoops)
	for i := 0; i < length; i++ {
		sw.AddAndRun(s.workLoops[i].Run)
	}

	// 启动主 Reactor
	sw.AddAndRun(s.listener.Run)
	s.running.Set(true)
	sw.Wait()
}

// Stop 关闭 Server
func (s *Server) Stop() {
	if s.running.Get() {
		s.running.Set(false)

		s.timingWheel.Stop()
		if err := s.listener.Stop(); err != nil {
			log.Error(err)
		}

		for k := range s.workLoops {
			if err := s.workLoops[k].Stop(); err != nil {
				log.Error(err)
			}
		}
	}

}

// Options 返回 options
func (s *Server) Options() Options {
	return *s.opts
}
