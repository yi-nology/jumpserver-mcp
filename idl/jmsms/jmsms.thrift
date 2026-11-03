/**
 * jmsms —— 管理面的**契约层**。
 *
 * 管理面回答「堡垒机里有什么、谁有权限」。它造出来给调用方看的形状有好几类，
 * 这里定义其中**稳定的**那些：一旦发布就不能悄悄改。
 *
 * 规矩与执行面一致（见 idl/jmsexec/jmsexec.thrift）：
 *
 *   - 形状稳定 → 进这份 IDL，工具产出它，服务器按它校验输出；
 *   - 形状是堡垒机自己的原始记录（资产详情里那些嵌套对象）→ 留在 map 里，
 *     键名在类型注释里逐个列出来。把动态的东西塞进契约，契约就不再是契约。
 *
 * 列表类工具共用的「计数」那三个字段不在这里：它们是 jms.ListMeta
 * （共享的 IDL，两个面共用一份），本文件只补管理面自己的词汇。
 *
 * 这些形状按域拆成同目录下的多份 thrift（同 namespace，生成同一个
 * Go 包，类型名不变）：
 *
 *   refs.thrift       各域共用的引用形状
 *   records.thrift    流水与摘要（会话/终端/命令审计/登录/操作/口令/传输）
 *   evidence.thrift   证据与会话实录
 *   touch.thrift      摸底
 *   permission.thrift 授权
 *   asset.thrift      资产与树节点、平台、标签
 *   user.thrift       用户、用户组、口令与 SSH 密钥
 *   account.thrift    资产上的登录账号
 *   automation.thrift 账号自动化
 *   gather.thrift     账号采集与落地
 *   job.thrift        批量作业
 *   delegation.thrift 委派
 *   network.thrift    网络：区域、网关、路径、连通性
 *   rbac.thrift       角色与 RBAC 角色矩阵
 *   security.thrift   登录 ACL、脱敏、剪贴板 ACL
 *   cmdfilter.thrift  命令过滤
 *   status.thrift     状态与诊断
 *
 * 本文件是入口：只 include 上面那些文件，本身不再定义类型。
 *
 * 生成方式（slim 模板：不引入 thrift 运行时，只出结构体）:
 *   ./script/gen-idl.sh（thriftgo --gen go:template=slim -r，-r 连同
 *   include 的文件一起生成，缺了它 include 的类型没有生成物）
 */
namespace go jmsms
include "refs.thrift"
include "records.thrift"
include "evidence.thrift"
include "touch.thrift"
include "permission.thrift"
include "asset.thrift"
include "user.thrift"
include "account.thrift"
include "automation.thrift"
include "gather.thrift"
include "job.thrift"
include "delegation.thrift"
include "network.thrift"
include "rbac.thrift"
include "security.thrift"
include "cmdfilter.thrift"
include "status.thrift"

/**
 * 资产摘要：资产类工具共用的那一份「这台机器是什么」。
 *
 * 必填的三项（id / name / address）是每一行都有的；其余按工具给不给。
 * accounts 是**账号个数**，不是账号列表 —— 账号列表在 jms_accounts_list 里。
 */

/*
 * 管理面 97 个工具的答案**全部**有具名契约类型，工具也按那个类型产出：
 * render.Object 归零，列表的**行**也不再是自由 map。
 *
 * 列表类工具共用一个信封（jms.ListMeta 的三个计数字段 + items + note）：
 *
 *   count     本次返回了几条
 *   total     过滤后命中了几条
 *   truncated 返回数是否少于命中数
 *   items     那些行（每一行都有自己的具名类型）
 *   note      截断了或者没匹配上时的一句话说明
 *
 * 诊断类工具的答复 = 共享的 Diagnosis（checks/findings/status/verdict/advice）
 * + 该工具自己的语境字段；那份组合类型在 biz/jumpserver/model/jmsms/outputs.go
 * 里手写，因为它由几个 IDL 词拼成，不是 IDL 的 struct 能自己说的。
 */
