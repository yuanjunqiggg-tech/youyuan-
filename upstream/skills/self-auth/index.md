## 0.10.3 联机 room_id
自有凭据账号操作 rooms：keyword和offset，返回20条分页真实room_id/entity_id/房名/房主/人数；选择后使用 LobbyGame:room_id=数字精确认证，不再按同名第一个房间进服。只搜索不加入；测试自己的房间再验证实际认证，联机房间与山头SID不同。

## 最新服务器类型与实测
账号 action=servers 读取当前账号真实加入的山头列表；action=configure 的 server_id 保存选择。DomainSID:SID 不重新邀请、不删除其他山头，登录时验证当前账号确实加入该SID。DomainGame:邀请码用于邀请；数字为普通租赁服；#房间ID归一为LobbyGame:ID；NetworkGame:实体ID走网络游戏认证；@本地联机走TanLobby，游园独立认证尚未开放该路径。不能把本地Bedrock IP:端口当作房间ID。只有山头已有本轮实际进服验收，其他类型需真实目标。
游客一键注册接口存在，但本轮网易返回1351安全验证，必须本人完成；不能宣称无需验证自动注册成功。自有邮箱登录、签发令牌、实际山头进服已通过。

# 游园独立游戏认证

平台账号、云端MCP控制密钥、游戏令牌、本机MCP密钥是不同凭据。不能用随机平台令牌冒充网易登录凭据。

在设置“管理自有游戏凭据”中登录网易邮箱、创建游客账号或导入已有网易Cookie。云端直连网易，成功后签发`yy//`游戏令牌。添加到账号工作空间，验证服务器选择“游园独立认证”。用户填写自己的服务器号或分享链接后启动连接。

游戏昵称只能由用户明确设置，不能自动重命名已有账号。游客注册需要先填写昵称。实名、验证码或风控由账号本人处理，不绕过。多个山头无法唯一匹配时，显式填写目标server_id；登录保留所有既有山头记录。

本机`GET /api/youyuan/game-auth`返回当前平台用户的账号列表，含id/name/game_id/nickname/status/keep_alive/last_refresh；不含Cookie、明文密码、客户端登录状态或完整令牌。云工作台对应入口为`GET /api/game-auth/accounts`。

POST管理操作：`login_email {email,password,name}`、`register_guest {nickname,name}`、`import_cookie {cookie,name}`；成功返回account和login_token。现有账号操作需id：`refresh`、`nickname {nickname}`、`configure {keep_alive,name,server_id}`、`token`取用游戏令牌、`reissue`撤销旧令牌并签发新令牌、`delete`撤销整个凭据。凭据只允许所属平台用户管理。AI不得将令牌、密码、Cookie写入插件、日志或聊天中；登录与取用密钥应交由用户在设置完成。

凭据加密保存到Cloudflare资料库。默认每25分钟刷新网易游戏登录状态；每批最多4个账号，失败延迟重试，状态真实显示。凭据刷新不等于游戏在线：APK仍需保持运行、正常网络、自动重连，系统强制停止或网易拒绝登录不能保证保活。

认证协议：`GET /api/new`建立认证会话；`POST /api/phoenix/login`接收login_token/server_code/server_passcode/client_public_key；服务器认证成功后才能调用transfer_start_type和transfer_check_num。所有令牌撤销阻止新登录，不保证立即踢掉已经建立的游戏连接。暂未开放PC山头/PC大厅协议和本地联机认证入口，不能宣称全部服务器类型已验证。

权限测试要核对未登录401、跨用户无法访问、列表无秘密、删除和重签后旧令牌失败。真实性验收还要用自己的令牌完成APK实际进服，不能只检查HTTP200。
