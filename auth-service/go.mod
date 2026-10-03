module youyuan/auth-service

go 1.27.0

require (
	github.com/Yeah114/FunAuth v0.0.0-00010101000000-000000000000
	github.com/Yeah114/g79client v0.0.0-00010101000000-000000000000
)

require (
	github.com/Yeah114/unmcpk v0.0.0-00010101000000-000000000000 // indirect
	github.com/database64128/chacha8-go v0.0.0-20250815115417-e0f2726d8bd0 // indirect
	github.com/google/uuid v1.6.0 // indirect
)

replace github.com/Yeah114/FunAuth => ../third_party/funauth

replace github.com/Yeah114/g79client => ../third_party/funauth/modules/g79client

replace github.com/Yeah114/unmcpk => ../third_party/funauth/modules/unmcpk
