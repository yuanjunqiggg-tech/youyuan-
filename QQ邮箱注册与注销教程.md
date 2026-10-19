# QQ邮箱注册与账号注销 · 0.10.22

注册平台账号：填写3–32字符账号名称、自己的QQ邮箱（@qq.com）和10–128字符密码，点击发送验证码，在邮箱收件箱/垃圾箱查收6位验证码，输入后注册。验证码只对应本次账号名和邮箱；10分钟过期、一次有效，错误5次必须重发，60秒后可重发。真实SMTP未接受时不显示发送成功，也不会创建账号。

`POST /api/identity/register-code`：`{username,email}`；成功返回expires_in、retry_after，不返回验证码。`POST /api/identity/register`：`{username,email,email_code,password}`。APK/Windows本机通过`POST /api/youyuan/identity`加上`action`调用相同流程。旧账号名/密码登录保留，旧QQ号不会被当作已经验证的邮箱。

退出登录只结束当前会话，平台账号仍然存在。真正注销在设置→游园平台账号→注销平台账号，输入当前密码并确认；云端撤销所有会话、设备和控制凭证、OAuth和MCP授权并释放用户名/邮箱。APK清除对应平台绑定；本机游戏账号、插件源码和数据不删除。注册后是新用户，原订阅和云端资料不会自动迁移。项目所有者需要先交接，不能自助注销。云端暂时离线不会自动清除本机归属。

邮箱发信运行在Linux执行端，Cloudflare仅校验验证码和签名转发。配置文件`/var/lib/youyuan/mail-config.json`须由youyuan用户拥有并设0600：smtp_host、smtp_port（465隐式TLS或587强制STARTTLS）、smtp_username、smtp_authorization_code、relay_secret。relay_secret对应本站私有CONSOLE_TOKEN。配置只在服务器读取，不打入APK、不公开、不返回给MCP。QQ邮箱开启SMTP后使用授权码，而非QQ登录密码。应用接口不能用来任意发邮件：只接受本站签名、限定QQ收件邮箱和6位注册验证码。

AI或MCP不得把退出描述成注销，不得在注册冲突时直接删除已有用户，不得以未验证的号码声称邮箱归属。管理员可以重置账号密码；密码只保存哈希，不支持反查原密码。


## 2026-10-09 注册闭环与项目管理授权

独立认证网站与工具箱共用同一个游园身份资料库。注册必须先 `POST /api/identity/register-code {username,email}`，再 `POST /api/identity/register {username,email,email_code,password}`。旧QQ号表单已替换；用户名3–32字符，密码10–128字符。发码、注册和登录分开显示结果，错误保留填写内容。平台账号注册不等于网易游戏账号注册，也不等于云端订阅授权。

网易游客遇到1351本人安全验证时，待验证设备和已取得的Cookie加密保存30分钟；不会保存网易明文密码。`GET /api/accounts`返回pending的公开id/name/nickname/expires。本人验证后`POST /api/accounts {action:"resume_guest",id}`继续同一设备；`cancel_guest`取消，过期返回410。每用户最多4个同时待验证任务，恢复仍遵守认证重试频率。服务器拒绝或要求实名时不能假装成功。

用户在创建游戏凭据时可勾选“授权项目管理员调用这个游戏账号”，后续通过`configure {id,allow_project_admin:false}`撤销。旧账号默认不授权。项目所有者登录认证网站后，项目管理区域可选平台用户，查看其授权的游戏账号，刷新、读山头/房间及取用令牌。`GET /api/admin/accounts`列用户，带user_id查询授权账号；POST带user_id/id/action调用，只开放token/refresh/servers/rooms/configure。不能修改用户授权、创建账号、读取网易密码或Cookie。

管理员token操作同时返回24小时有效的auth_url专用认证地址，填入管理员工具箱对应账号的验证服务器；仍要求管理员设备身份建立认证会话。授权地址只绑定一个用户的一个游戏账号，每次认证检查平台用户存在、账号授权、令牌未轮换和有效期；不能用一个账号的会话登录另一个账号。撤销授权、撤销/轮换游戏令牌或注销平台账号后拒绝新认证请求，不承诺强制踢掉已建立的游戏连接。不要把专用认证地址和游戏令牌写进公开日志、技能或插件源码。

注册及授权专项5项测试通过，覆盖真实前端提交到共享身份处理器（邮件投递使用测试桩）、网易安全验证后同设备恢复、密文保存、管理员隔离/撤销/防伪造及授权认证会话。公网旧表单已复现实测拒绝；实际游客返回1351，未标记注册成功；邮箱真实验证码完成注册和该游客真实进服仍需本人验证。
