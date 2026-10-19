# YouYuan wide-register fork
Based on GopherLua v1.1.2; original MIT LICENSE retained.
Single-function compiler register limit: 5120. Internal instructions: uint64,
47 significant bits; register and parameter metadata: uint16.
Lua source syntax/API/module state remains Lua 5.1. Upstream serialized
FunctionProto/bytecode and native Go code constructing []uint32 Code are
incompatible; compile source through this engine. RegistrySize=5120 is only
the initial data stack; the host enables bounded dynamic growth for nested
large frames. Raising the compiler limit is not a command throughput claim.
vm.go and _vm.go are patched together: do not regenerate using unmodified
upstream go-inline. Reverting go.mod replace restores the upstream engine.
