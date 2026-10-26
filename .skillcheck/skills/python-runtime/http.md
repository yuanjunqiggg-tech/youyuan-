# http · 网络与JSON

返回status/body/headers，非2xx响应也可检查，不把远端错误吞掉。


### `http.delete`

```python
host.http.delete(url, opts)
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.get`

```python
host.http.get(url, opts)
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.json_decode`

```python
host.http.json_decode(s)
```

- 作用：把 JSON 字符串解析为 JSON兼容值。
- 参数：`s` string 必填。
- 返回：任意 JSON兼容值。
- 报错：解析失败抛 `http.json_decode: <err>`。
- 底层：同 `game.jsonDecode`——把 JSON 解析成对应 Lua 类型（对象→表、数组→1 起索引的表）。

### `http.json_encode`

```python
host.http.json_encode(v)
```

- 作用：把 JSON兼容值转成 JSON 字符串。
- 参数：`v` 任意 JSON兼容值。
- 返回：string。
- 报错：序列化失败抛 `http.json_encode: <err>`。
- 底层：同 `game.jsonEncode`——把 JSON兼容值映射成 JSON 类型后编码成字符串。

### `http.post`

```python
host.http.post(url, body, opts)
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.put`

```python
host.http.put(url, body, opts)
```

- 作用：对应方法的 HTTP 请求。
- 参数：同 `request`（get/delete 无 body）。
- 返回：同 `request` 的响应表。
- 报错：无（错误进 `error` 字段）。
- 底层：`pushHTTPResult(L, "GET/POST/PUT/DELETE", ...)` → `httpRequestFull`。

### `http.querify`

```python
host.http.querify(paramsTable)
```

- 作用：把参数表编码成 URL query 字符串。
- 参数：`paramsTable` 对象或数组 必填（键值都转字符串）。
- 返回：string，如 `"a=1&b=2"`（按键排序，同键累加）。
- 报错：非 对象或数组 抛类型错误。
- 底层：`url.Values` + `vals.Encode()`。

> 与 `game.httpGet/httpPost` 的区别：`game.*` 返回纯文本、非 200 直接抛错，适合简单场景；`http.*` 返回完整响应对象、携带状态码与响应头、非 2xx 也能拿 body，适合需要细粒度控制的场景。

### `http.request`

```python
host.http.request(method, url, body, opts)
```

- 作用：发送任意 method 的 HTTP 请求，返回完整响应对象。
- 参数：`method` string 必填；`url` string 必填；`body` string 可选，默认 `""`；`opts` 对象或数组 可选（含 `headers` 表、`timeout` 秒）。
- 返回：对象或数组。成功 `{ok=true, status=number, body=string, headers=对象或数组}`；传输错误 `{ok=false, error=string}`。
- 报错：无（错误放进 `error` 字段，不抛异常）。
- 底层：`httpRequestFull(method, u, body, headers, timeout)`。
