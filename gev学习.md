# gev 学习

> gev 是一个轻量、快速的基于 Reactor 模式的非阻塞 TCP 网络库 / websocket server，支持自定义协议，轻松快速搭建高性能服务器。

特色

- 基于 epoll 和 kqueue 实现的高性能事件循环
- 支持多核多线程
- 动态扩容 Ring Buffer 实现的读写缓冲区
- 异步读写
- 自动清理空闲连接
- SO_REUSEPORT 端口重用支持
- 支持 WebSocket/Protobuf, 自定义协议
- 支持定时任务，延时任务
- 开箱即用的高性能 websocket server

gev 使用类似 redis 的 reactor 网络模型处理客户端的读写事件\




## 快速开始
