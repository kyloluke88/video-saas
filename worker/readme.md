## 完整流程图
现在 worker 启动后，顺序是这样的：

connectAndPrepare()
conn := amqp.Dial(...)
开一个临时 ch
setupTopology(ch)
关闭这个临时 ch
返回 conn
然后：

consumer pool 启动多个 goroutine
每个 goroutine 再调用 conn.Channel()
得到自己的消费 ch
ch.Consume(...)
收到消息
调 dispatcher.HandleMessage(ch, msg)