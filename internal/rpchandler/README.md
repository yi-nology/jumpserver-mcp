# internal/rpchandler/ —— RPC 的服务端

**⚠️ 这一层是空的，而且它是故意空的。**

泛型的 RPC 面住在**底座的 `pkg/rpcapi`**（kitex/thrift；契约在 kit 的
`idl/jmsrpc` + `kitex_gen/jmsrpc`），两个服务共用同一扇门、同一道门禁、同一份留痕。

`⚠️ 什么时候它不再是空的：某个工具需要一个**不适合泛型 Invoke** 的服务专属 RPC
方法，就在这一层放那个方法的服务端——它有自己的 IDL 与版本，守卫会盯住它与
泛型面的边界。谁在用：留痕的 caller 列（rpc/自报姓名）。
`
