# 抽卡模拟器 (Gacha Simulator) - RESTful API 接口文档

本接口文档详细说明抽卡模拟器后端服务提供的全部 RESTful API 接口，包括权限鉴权、请求规范、入参与返回值定义、示例调用（cURL）及错误响应格式。

---

## 目录
- [1. 概览与认证规范](#1-概览与认证规范)
  - [1.1 服务基础信息](#11-服务基础信息)
  - [1.2 认证机制 (JWT Bearer Token)](#12-认证机制-jwt-bearer-token)
  - [1.3 默认管理员账号](#13-默认管理员账号)
  - [1.4 通用错误响应规范](#14-通用错误响应规范)
- [2. 用户与认证模块 (Auth & User Module)](#2-用户与认证模块-auth--user-module)
  - [2.1 账号注册 (POST /api/register)](#21-账号注册-post-apiregister)
  - [2.2 账号密码登录 (POST /api/login)](#22-账号密码登录-post-apilogin)
  - [2.3 发送邮箱验证码 (POST /api/auth/email/send-code)](#23-发送邮箱验证码-post-apiauthemailsend-code)
  - [2.4 邮箱验证码登录/自动注册 (POST /api/auth/email/login)](#24-邮箱验证码登录自动注册-post-apiauthemaillogin)
  - [2.5 获取 GitHub 授权链接 (GET /api/auth/github/login)](#25-获取-github-授权链接-get-apiauthgithublogin)
  - [2.6 GitHub 授权回调 (GET/POST /api/auth/github/callback)](#26-github-授权回调-getpost-apiauthgithubcallback)
  - [2.7 获取当前登录用户信息 (GET /api/user/me)](#27-获取当前登录用户信息-get-apiuserme)
  - [2.8 修改个人资料 (PUT /api/user/profile)](#28-修改个人资料-put-apiuserprofile)
  - [2.9 退出登录 (POST /api/user/logout)](#29-退出登录-post-apiuserlogout)
  - [2.10 每日星穹占卜 (POST /api/user/divination)](#210-每日星穹占卜-post-apiuserdivination)
  - [2.11 查询今日占卜结果 (GET /api/user/divination)](#211-查询今日占卜结果-get-apiuserdivination)
- [3. 抽卡与玩法模块 (Gacha & Gameplay Module)](#3-抽卡与玩法模块-gacha--gameplay-module)
  - [3.1 获取卡池与概率配置 (GET /api/pool/info)](#31-获取卡池与概率配置-get-apipoolinfo)
  - [3.2 抽卡 (单抽 / 十连抽) (POST /api/gacha/draw)](#32-抽卡-单抽--十连抽-post-apigachadraw)
  - [3.3 查询用户背包与持有角色 (GET /api/gacha/inventory)](#33-查询用户背包与持有角色-get-apigachainventory)
  - [3.4 分页查询抽卡历史 (GET /api/gacha/history)](#34-分页查询抽卡历史-get-apigachahistory)
  - [3.5 S 档出金统计与歪卡分析 (GET /api/gacha/stats)](#35-s-档出金统计与歪卡分析-get-apigachastats)
  - [3.6 清空历史背包并重置保底 (DELETE /api/gacha/history)](#36-清空历史背包并重置保底-delete-apigachahistory)
  - [3.7 实时事件通知流 (GET /api/notifications)](#37-实时事件通知流-get-apinotifications)
  - [3.8 蒙特卡洛抽卡极速测算 (POST /api/gacha/simulate)](#38-蒙特卡洛抽卡极速测算-post-apigachasimulate)
- [4. 管理员后台模块 (Admin Module)](#4-管理员后台模块-admin-module)
  - [4.1 创建新角色 (POST /api/admin/character)](#41-创建新角色-post-apiadmincharacter)
  - [4.2 推送角色入卡池 (POST /api/admin/pool/push)](#42-推送角色入卡池-post-apiadminpoolpush)
  - [4.3 批量加载预设卡池 (POST /api/admin/pool/load-presets)](#43-批量加载预设卡池-post-apiadminpoolload-presets)
  - [4.4 动态热更新卡池配置 (PUT /api/admin/pool/config)](#44-动态热更新卡池配置-put-apiadminpoolconfig)
- [5. 数据模型定义 (Data Models)](#5-数据模型定义-data-models)
- [6. gRPC 配置流式分发服务 (gRPC Config Streaming Service)](#6-grpc-配置流式分发服务-grpc-config-streaming-service)
- [7. 服务解耦架构与运行模式 (Architecture Decoupling & Server Run Modes)](#7-服务解耦架构与运行模式-architecture-decoupling--server-run-modes)
  - [7.1 架构拆分与职责划分](#71-架构拆分与职责划分)
  - [7.2 命令行启动模式与参数](#72-命令行启动模式与参数)
  - [7.3 Ed25519 非对称密钥与跨服务免密鉴权](#73-ed25519-非对称密钥与跨服务免密鉴权)
- [8. CLI 终端客户端使用指南 (Terminal CLI Client Guide)](#8-cli-终端客户端使用指南-terminal-cli-client-guide)
  - [8.1 编译与快速上手](#81-编译与快速上手)
  - [8.2 交互式 REPL 菜单模式](#82-交互式-repl-菜单模式)
  - [8.3 非交互式直接指令模式](#83-非交互式直接指令模式)
  - [8.4 本地凭证持久化与高亮输出](#84-本地凭证持久化与高亮输出)

---

## 1. 概览与认证规范

### 1.1 服务基础信息
- **Base URL**: `http://localhost:8080`
- **协议**: HTTP/1.1
- **数据传输格式**: `application/json`

### 1.2 认证机制 (JWT Bearer Token)
后端使用 Ed25519 (EdDSA) 签名生成 JSON Web Token (JWT)，有效期为 24 小时。
保护受保护端点（Protected Endpoints）时，客户端必须在 HTTP 请求头中携带 Token：

```http
Authorization: Bearer <JWT_TOKEN>
```

未携带 Token 或 Token 无效/过期时，系统将返回 `401 Unauthorized` 状态码。

### 1.3 默认管理员账号
系统首次启动并完成数据库迁移时，若不存在任何 `role = 'admin'` 的用户，将自动初始化默认管理员：
- **账号 ID**: `admin`
- **密码**: `admin123`
- **角色**: `admin`

### 1.4 通用错误响应规范
发生客户端或服务端错误时，接口返回统一的 JSON 错误对象：

```json
{
  "error": "详细错误信息描述"
}
```

常见 HTTP 状态码说明：
| HTTP 状态码 | 含义 | 触发场景 |
| :--- | :--- | :--- |
| `200 OK` | 请求成功 | 接口正常处理完毕 |
| `307 Temporary Redirect` | 临时重定向 | OAuth 授权跳转 |
| `400 Bad Request` | 请求参数错误 | 必填字段缺失、格式不符合校验规则等 |
| `401 Unauthorized` | 鉴权失败 | 未登录、账号密码错误、Token 过期或格式不符 |
| `403 Forbidden` | 权限不足 | 普通用户尝试访问 Admin 接口 |
| `404 Not Found` | 资源未找到 | 目标用户或目标角色不存在 |
| `500 Internal Server Error` | 服务端内部错误 | 数据库异常、抽卡奖池空异常等 |

---

## 2. 用户与认证模块 (Auth & User Module)

### 2.1 账号注册 (POST /api/register)
- **接口路径**: `POST /api/register`
- **权限要求**: 公开 (Public)
- **接口描述**: 使用昵称、密码及可选个人简介注册新账号。系统自动为用户生成全局唯一的 8 位字符 ID。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Content-Type` | `application/json` | 是 |

#### 请求体 (Request Body)
| 字段名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `nickname` | `string` | 是 | 用户昵称 |
| `password` | `string` | 是 | 密码 (经 bcrypt 哈希后存入数据库) |
| `bio` | `string` | 否 | 个人简介 |

#### cURL 示例
```bash
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "nickname": "gacha_fan",
    "password": "secretpassword",
    "bio": "测试抽卡模拟器"
  }'
```

#### 成功响应 (200 OK)
```json
{
  "message": "registry successfully",
  "data": {
    "id": "73b6ae34",
    "nickname": "gacha_fan"
  }
}
```

#### 错误响应 (400 Bad Request)
```json
{
  "error": "wrong func"
}
```

---

### 2.2 账号密码登录 (POST /api/login)
- **接口路径**: `POST /api/login`
- **权限要求**: 公开 (Public)
- **接口描述**: 使用账号 ID 和密码进行登录验证，成功后颁发有效时长 24 小时的 JWT Token。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Content-Type` | `application/json` | 是 |

#### 请求体 (Request Body)
| 字段名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `id` | `string` | 是 | 用户 ID (如注册返回的 8 位 ID 或 `admin`) |
| `password` | `string` | 是 | 登录密码 |

#### cURL 示例
```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{
    "id": "73b6ae34",
    "password": "secretpassword"
  }'
```

#### 成功响应 (200 OK)
```json
{
  "message": "login successfully",
  "token": "eyJhbGciOiJFZERTQSI...",
  "user": {
    "id": "73b6ae34",
    "nickname": "gacha_fan",
    "role": "user",
    "bio": "测试抽卡模拟器"
  }
}
```

#### 错误响应 (401 Unauthorized)
```json
{
  "error": "wrong pwd"
}
```

---

### 2.3 发送邮箱验证码 (POST /api/auth/email/send-code)
- **接口路径**: `POST /api/auth/email/send-code`
- **权限要求**: 公开 (Public)
- **接口描述**: 向指定的合法邮箱地址发送 6 位数字验证码（有效时长 10 分钟）。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Content-Type` | `application/json` | 是 |

#### 请求体 (Request Body)
| 字段名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `email` | `string` | 是 | 目标邮箱地址 (需符合 Email 标准格式) |

#### cURL 示例
```bash
curl -X POST http://localhost:8080/api/auth/email/send-code \
  -H "Content-Type: application/json" \
  -d '{
    "email": "traveler@genshin.com"
  }'
```

#### 成功响应 (200 OK)
```json
{
  "message": "verification code sent successfully"
}
```

#### 错误响应 (400 Bad Request)
```json
{
  "error": "invalid email format"
}
```

---

### 2.4 邮箱验证码登录/自动注册 (POST /api/auth/email/login)
- **接口路径**: `POST /api/auth/email/login`
- **权限要求**: 公开 (Public)
- **接口描述**: 使用邮箱与验证码登录。如果邮箱尚未绑定任何账号，系统将自动创建新用户并以邮箱前缀作为初始昵称。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Content-Type` | `application/json` | 是 |

#### 请求体 (Request Body)
| 字段名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `email` | `string` | 是 | 邮箱地址 |
| `code` | `string` | 是 | 6 位有效验证码 |

#### cURL 示例
```bash
curl -X POST http://localhost:8080/api/auth/email/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "traveler@genshin.com",
    "code": "123456"
  }'
```

#### 成功响应 (200 OK)
```json
{
  "message": "login successfully",
  "token": "eyJhbGciOiJFZERTQSI...",
  "user": {
    "id": "e9a01f82",
    "nickname": "traveler",
    "role": "user",
    "bio": "",
    "email": "traveler@genshin.com"
  }
}
```

#### 错误响应 (400 Bad Request)
```json
{
  "error": "invalid or expired verification code"
}
```

---

### 2.5 获取 GitHub 授权链接 (GET /api/auth/github/login)
- **接口路径**: `GET /api/auth/github/login`
- **权限要求**: 公开 (Public)
- **接口描述**: 获取用于重定向至 GitHub OAuth 的授权登录 URL。若传参 `redirect=true`，则直接通过 HTTP 307 临时重定向跳转。

#### 查询参数 (Query Parameters)
| 参数名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `state` | `string` | 否 | CSRF 防护随机状态码 (若未提供，系统自动生成 8 位 UUID) |
| `redirect` | `string` | 否 | 若值为 `true`，服务端直接返回 307 重定向至 GitHub |

#### cURL 示例
```bash
curl -X GET "http://localhost:8080/api/auth/github/login?state=state_abc"
```

#### 成功响应 (200 OK，当 redirect 未指定或不为 true)
```json
{
  "auth_url": "https://github.com/login/oauth/authorize?client_id=Iv23liGWo2PtRjCjSTQ3&redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Fapi%2Fauth%2Fgithub%2Fcallback&scope=read%3Auser+user%3Aemail&state=state_abc",
  "state": "state_abc"
}
```

---

### 2.6 GitHub 授权回调 (GET/POST /api/auth/github/callback)
- **接口路径**: `GET /api/auth/github/callback` 或 `POST /api/auth/github/callback`
- **权限要求**: 公开 (Public)
- **接口描述**: 接收 GitHub 授权颁发的 `code`，换取 Access Token 并拉取 GitHub 用户资料。若为首次登录则自动绑定或创建用户，并颁发 JWT Token。

#### 请求参数 (Query 或 JSON / Form Body)
| 参数名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `code` | `string` | 是 | GitHub 授权 Code |
| `state` | `string` | 否 | 客户端发起请求时的状态校验值 |

#### cURL 示例
```bash
# 方式一：GET 查询参数
curl -X GET "http://localhost:8080/api/auth/github/callback?code=gh_oauth_code_123&state=state_abc"

# 方式二：POST JSON 请求体
curl -X POST http://localhost:8080/api/auth/github/callback \
  -H "Content-Type: application/json" \
  -d '{
    "code": "gh_oauth_code_123",
    "state": "state_abc"
  }'
```

#### 成功响应 (200 OK)
```json
{
  "message": "github login successfully",
  "token": "eyJhbGciOiJFZERTQSI...",
  "user": {
    "id": "c4d5e6f7",
    "nickname": "octocat",
    "role": "user",
    "bio": "",
    "email": "octocat@github.com",
    "github_id": "123456"
  }
}
```

#### 错误响应 (400 Bad Request)
```json
{
  "error": "authorization code is required"
}
```

---

### 2.7 获取当前登录用户信息 (GET /api/user/me)
- **接口路径**: `GET /api/user/me`
- **权限要求**: 需登录 (Protected, 任意角色)
- **接口描述**: 解析当前请求携带的 JWT Token，返回解析后的用户 ID 与权限角色。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Authorization` | `Bearer <JWT_TOKEN>` | 是 |

#### cURL 示例
```bash
curl -X GET http://localhost:8080/api/user/me \
  -H "Authorization: Bearer eyJhbGciOiJFZERTQSI..."
```

#### 成功响应 (200 OK)
```json
{
  "user_id": "73b6ae34",
  "role": "user"
}
```

#### 错误响应 (401 Unauthorized)
```json
{
  "error": "authorization failed"
}
```

---

### 2.8 修改个人资料 (PUT /api/user/profile)
- **接口路径**: `PUT /api/user/profile`
- **权限要求**: 需登录 (Protected)
- **接口描述**: 修改当前登录用户的昵称 (`nickname`) 或个人简介 (`bio`)。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Content-Type` | `application/json` | 是 |
| `Authorization` | `Bearer <JWT_TOKEN>` | 是 |

#### 请求体 (Request Body)
| 字段名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `nickname` | `string` | 否 | 新的用户昵称 (如不传或为空字符串则保持不变) |
| `bio` | `string` | 否 | 新的个人简介 (如不传或为空字符串则保持不变) |

#### cURL 示例
```bash
curl -X PUT http://localhost:8080/api/user/profile \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJFZERTQSI..." \
  -d '{
    "nickname": "new_traveler",
    "bio": "新世界探索者"
  }'
```

#### 成功响应 (200 OK)
```json
{
  "message": "updated succssfully",
  "data": {
    "id": "73b6ae34",
    "nickname": "new_traveler",
    "bio": "新世界探索者",
    "role": "user",
    "email": null,
    "github_id": null,
    "pity_s_count": 0,
    "pity_a_count": 0,
    "created_at": "2026-10-06T12:00:00Z",
    "updated_at": "2026-10-06T12:30:00Z"
  }
}
```

#### 错误响应 (401 Unauthorized)
```json
{
  "error": "Token 无效或已过期"
}
```

---

### 2.9 退出登录 (POST /api/user/logout)
- **接口路径**: `POST /api/user/logout`
- **权限要求**: 需登录 (Protected)
- **接口描述**: 退出当前登录状态。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Authorization` | `Bearer <JWT_TOKEN>` | 是 |

#### cURL 示例
```bash
curl -X POST http://localhost:8080/api/user/logout \
  -H "Authorization: Bearer eyJhbGciOiJFZERTQSI..."
```

#### 成功响应 (200 OK)
```json
{
  "message": "logged out!"
}
```

---

### 2.10 每日星穹占卜 (POST /api/user/divination)
- **接口路径**: `POST /api/user/divination`
- **权限要求**: 需登录 (Protected)
- **接口描述**: 用户每日抽取一次宇宙星象签文，获得命途神谕与星琼奖励。若今日已完成占卜，则直接返回今日已有占卜结果，不会重复发放奖励。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Authorization` | `Bearer <JWT_TOKEN>` | 是 |

#### cURL 示例
```bash
curl -X POST http://localhost:8080/api/user/divination \
  -H "Authorization: Bearer eyJhbGciOiJFZERTQSI..."
```

#### 首次占卜成功响应 (200 OK)
```json
{
  "message": "占卜完成",
  "already_drawn": false,
  "data": {
    "id": 1,
    "user_id": "73b6ae34",
    "date": "2026-10-08",
    "sign": "大吉·星神注视",
    "description": "星海泛起璀璨回音，今日十连必有金光闪烁！",
    "reward_amount": 160,
    "created_at": "2026-10-08T09:00:00Z"
  }
}
```

#### 今日已占卜重复调用响应 (200 OK)
```json
{
  "message": "今日已完成占卜",
  "already_drawn": true,
  "data": {
    "id": 1,
    "user_id": "73b6ae34",
    "date": "2026-10-08",
    "sign": "大吉·星神注视",
    "description": "星海泛起璀璨回音，今日十连必有金光闪烁！",
    "reward_amount": 160,
    "created_at": "2026-10-08T09:00:00Z"
  }
}
```

---

### 2.11 查询今日占卜结果 (GET /api/user/divination)
- **接口路径**: `GET /api/user/divination`
- **权限要求**: 需登录 (Protected)
- **接口描述**: 查询当前登录用户今日是否已完成占卜以及对应的签文记录。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Authorization` | `Bearer <JWT_TOKEN>` | 是 |

#### cURL 示例
```bash
curl -X GET http://localhost:8080/api/user/divination \
  -H "Authorization: Bearer eyJhbGciOiJFZERTQSI..."
```

#### 今日已占卜响应 (200 OK)
```json
{
  "message": "今日已完成占卜",
  "already_drawn": true,
  "data": {
    "id": 1,
    "user_id": "73b6ae34",
    "date": "2026-10-08",
    "sign": "大吉·星神注视",
    "description": "星海泛起璀璨回音，今日十连必有金光闪烁！",
    "reward_amount": 160,
    "created_at": "2026-10-08T09:00:00Z"
  }
}
```

#### 今日尚未占卜响应 (200 OK)
```json
{
  "message": "今日尚未占卜",
  "already_drawn": false,
  "data": null
}
```

---

## 3. 抽卡与玩法模块 (Gacha & Gameplay Module)

### 3.1 获取卡池与概率配置 (GET /api/pool/info)
- **接口路径**: `GET /api/pool/info`
- **权限要求**: 公开 (Public)
- **接口描述**: 查询全局卡池概率算法配置、当前 UP 角色、当前限定 S 档角色列表以及常驻卡池角色总数。

#### cURL 示例
```bash
curl -X GET http://localhost:8080/api/pool/info
```

#### 成功响应 (200 OK)
```json
{
  "config": {
    "base_rate_s": 0.008,
    "base_rate_a": 0.08,
    "base_rate_b": 0.912,
    "soft_pity_start": 65,
    "soft_pity_inc": 0.05,
    "hard_pity_s": 80,
    "hard_pity_a": 10,
    "max_limited_s": 3
  },
  "banner": {
    "up_character": {
      "id": 1,
      "name": "星渊猎手·卡莲",
      "rarity": "S",
      "is_limited": true,
      "in_pool": true,
      "is_up": true,
      "entered_pool_at": "2026-10-06T10:00:00Z"
    },
    "limited_s_character": [
      {
        "id": 1,
        "name": "星渊猎手·卡莲",
        "rarity": "S",
        "is_limited": true,
        "in_pool": true,
        "is_up": true,
        "entered_pool_at": "2026-10-06T10:00:00Z"
      },
      {
        "id": 2,
        "name": "虚数神机·天枢",
        "rarity": "S",
        "is_limited": true,
        "in_pool": true,
        "is_up": false,
        "entered_pool_at": "2026-10-06T10:00:00Z"
      }
    ],
    "standard_pool_count": 8
  }
}
```

---

### 3.2 抽卡 (单抽 / 十连抽) (POST /api/gacha/draw)
- **接口路径**: `POST /api/gacha/draw`
- **权限要求**: 需登录 (Protected)
- **接口描述**: 执行单抽 (`count=1`) 或十连抽 (`count=10`)。系统严格执行保底机制、重复角色命座累加 (`rank`) 并记录每抽保底消耗水位。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Content-Type` | `application/json` | 是 |
| `Authorization` | `Bearer <JWT_TOKEN>` | 是 |

#### 请求体 (Request Body)
| 字段名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `count` | `integer` | 是 | 抽卡次数，**必须且仅能为 1 或 10** |

#### cURL 示例
```bash
curl -X POST http://localhost:8080/api/gacha/draw \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJFZERTQSI..." \
  -d '{
    "count": 10
  }'
```

#### 成功响应 (200 OK)
```json
{
  "message": "抽卡完成！",
  "count": 1,
  "results": [
    {
      "character": {
        "id": 1,
        "name": "星渊猎手·卡莲",
        "rarity": "S",
        "is_limited": true,
        "in_pool": true,
        "is_up": true,
        "entered_pool_at": "2026-10-06T10:00:00Z"
      },
      "is_first_time": true,
      "rank": 0,
      "pity_count_s": 42
    }
  ]
}
```

#### 错误响应 (400 Bad Request - 次数不合法)
```json
{
  "error": "参数错误，抽卡次数 count 只能为 1 或 10"
}
```

#### 错误响应 (500 Internal Server Error - 卡池缺乏该档位角色)
```json
{
  "error": "抽卡失败: 卡池中暂无 A 档角色，请联系管理员补充"
}
```

---

### 3.3 查询用户背包与持有角色 (GET /api/gacha/inventory)
- **接口路径**: `GET /api/gacha/inventory`
- **权限要求**: 需登录 (Protected)
- **接口描述**: 查询当前登录用户背包持有的全部角色列表及当前命座等级（`rank`，初次获得为 0，每重复抽到一次累加 1）。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Authorization` | `Bearer <JWT_TOKEN>` | 是 |

#### cURL 示例
```bash
curl -X GET http://localhost:8080/api/gacha/inventory \
  -H "Authorization: Bearer eyJhbGciOiJFZERTQSI..."
```

#### 成功响应 (200 OK)
```json
{
  "total": 2,
  "data": [
    {
      "id": 1,
      "user_id": "73b6ae34",
      "character_id": 1,
      "character": {
        "id": 1,
        "name": "星渊猎手·卡莲",
        "rarity": "S",
        "is_limited": true,
        "in_pool": true,
        "is_up": true,
        "entered_pool_at": "2026-10-06T10:00:00Z"
      },
      "rank": 1
    },
    {
      "id": 2,
      "user_id": "73b6ae34",
      "character_id": 4,
      "character": {
        "id": 4,
        "name": "影刃特工·夜枭",
        "rarity": "A",
        "is_limited": false,
        "in_pool": true,
        "is_up": false,
        "entered_pool_at": "2026-10-06T10:00:00Z"
      },
      "rank": 0
    }
  ]
}
```

---

### 3.4 分页查询抽卡历史 (GET /api/gacha/history)
- **接口路径**: `GET /api/gacha/history`
- **权限要求**: 需登录 (Protected)
- **接口描述**: 分页查询用户历史抽卡记录，按抽卡时间倒序排列。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Authorization` | `Bearer <JWT_TOKEN>` | 是 |

#### 查询参数 (Query Parameters)
| 参数名 | 类型 | 必填 | 默认值 | 约束 | 描述 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `page` | `integer` | 否 | `1` | `>= 1` | 当前页码 |
| `page_size` | `integer` | 否 | `20` | `1 ~ 100` | 每页记录数 |

#### cURL 示例
```bash
curl -X GET "http://localhost:8080/api/gacha/history?page=1&page_size=10" \
  -H "Authorization: Bearer eyJhbGciOiJFZERTQSI..."
```

#### 成功响应 (200 OK)
```json
{
  "total": 1,
  "page": 1,
  "page_size": 10,
  "data": [
    {
      "id": 1,
      "user_id": "73b6ae34",
      "character_id": 1,
      "character_name": "星渊猎手·卡莲",
      "rarity": "S",
      "is_first_time": true,
      "pity_count": 42,
      "created_at": "2026-10-06T12:05:00Z"
    }
  ]
}
```

---

### 3.5 S 档出金统计与歪卡分析 (GET /api/gacha/stats)
- **接口路径**: `GET /api/gacha/stats`
- **权限要求**: 需登录 (Protected)
- **接口描述**: 筛选用户历史上所有抽取到的 S 档角色，统计总出金数、每金花费的保底水位（`pulls_taken`）及是否歪卡（`is_won` 标签）。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Authorization` | `Bearer <JWT_TOKEN>` | 是 |

#### cURL 示例
```bash
curl -X GET http://localhost:8080/api/gacha/stats \
  -H "Authorization: Bearer eyJhbGciOiJFZERTQSI..."
```

#### 成功响应 (200 OK)
```json
{
  "total_s_count": 2,
  "history": [
    {
      "character_name": "星渊猎手·卡莲",
      "pulls_taken": 42,
      "is_up": true,
      "is_won": "拿下 UP！未歪！",
      "pulled_at": "2026-10-06 12:05:00"
    },
    {
      "character_name": "深空观测者·阿尔法",
      "pulls_taken": 78,
      "is_up": false,
      "is_won": "歪了！",
      "pulled_at": "2026-10-06 12:30:15"
    }
  ]
}
```

---

### 3.6 清空历史背包并重置保底 (DELETE /api/gacha/history)
- **接口路径**: `DELETE /api/gacha/history`
- **权限要求**: 需登录 (Protected)
- **接口描述**: 在单一数据库事务中清空当前用户的全部抽卡历史记录、清空背包中的角色，并将用户的 S 档与 A 档保底计数重置为 0。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Authorization` | `Bearer <JWT_TOKEN>` | 是 |

#### cURL 示例
```bash
curl -X DELETE http://localhost:8080/api/gacha/history \
  -H "Authorization: Bearer eyJhbGciOiJFZERTQSI..."
```

#### 成功响应 (200 OK)
```json
{
  "message": "已成功抹除所有非酋记录与角色，保底计数已归零！重新出发吧！"
}
```

---

### 3.7 实时事件通知流 (GET /api/notifications)
- **接口路径**: `GET /api/notifications`
- **权限要求**: 公开 (Public)，无需鉴权
- **协议**: Server-Sent Events (SSE, RFC 8895)
- **Content-Type**: `text/event-stream`
- **接口描述**: 客户端连接此端点可建立长连接，实时接收卡池变更事件（`POOL_UPDATE`）及全局概率热重载事件（`PROB_UPDATE`）。连接建立后服务器立即推送 `CONNECTED` 握手事件。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Accept` | `text/event-stream` | 推荐 |

#### SSE 事件格式说明
| 事件名 (`event`) | 数据说明 (`data`) | 触发场景 |
| :--- | :--- | :--- |
| `CONNECTED` | `{"event":"CONNECTED","data":"SSE notification stream connected","timestamp":...}` | 客户端首次连接成功 |
| `POOL_UPDATE` | `{"event":"POOL_UPDATE","data":"角色【...】已加入卡池！","timestamp":...}` | 管理员推送角色入池或加载预设卡池 |
| `PROB_UPDATE` | `{"event":"PROB_UPDATE","data":"卡池概率配置已更新！","timestamp":...}` | 管理员通过接口修改全局概率与保底配置 |

#### cURL 示例
```bash
curl -N -H "Accept: text/event-stream" http://localhost:8080/api/notifications
```

#### 数据流响应示例
```text
event: CONNECTED
data: {"event":"CONNECTED","data":"SSE notification stream connected","timestamp":1791338400}

event: POOL_UPDATE
data: {"event":"POOL_UPDATE","data":"角色【星渊猎手·卡莲】已加入卡池！","timestamp":1791338405}

event: PROB_UPDATE
data: {"event":"PROB_UPDATE","data":"卡池概率配置已更新！","timestamp":1791338410}
```

---

### 3.8 蒙特卡洛抽卡极速测算 (POST /api/gacha/simulate)
- **接口路径**: `POST /api/gacha/simulate`
- **权限要求**: 需登录 (Protected)
- **接口描述**: 基于当前全局活跃的概率与保底配置快照（软保底加成、硬保底阈值、50% UP 不歪机制），进行纯内存的大规模蒙特卡洛随机抽样仿真测算（1 ~ 50,000 次跃迁），零数据库写入污染，毫秒级返回样本出率、出金平均间隔与欧皇指数评分。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Content-Type` | `application/json` | 是 |
| `Authorization` | `Bearer <JWT_TOKEN>` | 是 |

#### 请求体 (Request Body)
| 字段名 | 类型 | 必填 | 默认值 | 描述 |
| :--- | :--- | :--- | :--- | :--- |
| `pulls` | `int` | 否 | 1000 | 拟测算的抽卡总次数 (范围: 1 ~ 50000) |

#### cURL 示例
```bash
curl -X POST http://localhost:8080/api/gacha/simulate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJFZERTQSI..." \
  -d '{
    "pulls": 1000
  }'
```

#### 成功响应 (200 OK)
```json
{
  "total_pulls": 1000,
  "s_count": 16,
  "a_count": 128,
  "b_count": 856,
  "up_count": 9,
  "non_up_count": 7,
  "empirical_s_rate": 1.6,
  "empirical_a_rate": 12.8,
  "avg_pulls_per_s": 62.5,
  "up_rate": 56.25,
  "luck_score": 72,
  "luck_level": "欧气满满",
  "cfg_snapshot": {
    "base_rate_s": 0.008,
    "base_rate_a": 0.08,
    "base_rate_b": 0.912,
    "soft_pity_start": 65,
    "soft_pity_inc": 0.05,
    "hard_pity_s": 80,
    "hard_pity_a": 10,
    "max_limited_s": 3
  }
}
```

#### 错误响应 (400 Bad Request)
```json
{
  "error": "单次模拟抽卡次数不可超过 50,000"
}
```

---

## 4. 管理员后台模块 (Admin Module)

> **注意**: 管理员模块下的全部接口均需要用户具备 `role = 'admin'` 权限。若普通用户访问，将收到 `403 Forbidden`。

### 4.1 创建新角色 (POST /api/admin/character)
- **接口路径**: `POST /api/admin/character`
- **权限要求**: 管理员 (Role: admin)
- **接口描述**: 向数据库中创建新角色。新创建的角色默认不在卡池中（`in_pool = false`），需后续通过入池接口推入。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Content-Type` | `application/json` | 是 |
| `Authorization` | `Bearer <ADMIN_JWT_TOKEN>` | 是 |

#### 请求体 (Request Body)
| 字段名 | 类型 | 必填 | 约束 | 描述 |
| :--- | :--- | :--- | :--- | :--- |
| `name` | `string` | 是 | 字符长度不超过 50 | 角色名称 |
| `rarity` | `string` | 是 | 仅允许 `"S"`、`"A"`、`"B"` | 角色稀有度档位 |
| `is_limited` | `boolean` | 否 | 默认 `false` | 是否为限定角色 |

#### cURL 示例
```bash
curl -X POST http://localhost:8080/api/admin/character \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <ADMIN_JWT_TOKEN>" \
  -d '{
    "name": "极光战姬·艾莉亚",
    "rarity": "S",
    "is_limited": true
  }'
```

#### 成功响应 (200 OK)
```json
{
  "message": "character created successfully",
  "data": {
    "id": 11,
    "name": "极光战姬·艾莉亚",
    "rarity": "S",
    "is_limited": true,
    "in_pool": false,
    "is_up": false,
    "entered_pool_at": null
  }
}
```

#### 错误响应 (400 Bad Request - 稀有度不合法)
```json
{
  "error": "rarity must be S A or B"
}
```

#### 错误响应 (403 Forbidden - 非管理员)
```json
{
  "error": "permission denied"
}
```

---

### 4.2 推送角色入卡池 (POST /api/admin/pool/push)
- **接口路径**: `POST /api/admin/pool/push`
- **权限要求**: 管理员 (Role: admin)
- **接口描述**: 将角色推入卡池（设置 `is_in_pool = true`，更新 `entered_pool_at` 时间戳）。
  - 若将该角色设为 UP（`is_up = true`），会自动将先前 UP 状态的角色取消 UP。
  - 对于限定 S 档角色，若池内限定角色数量达到系统上限（默认 3 个），将遵循 FIFO 规则将入池时间最早的限定 S 角色移出卡池。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Content-Type` | `application/json` | 是 |
| `Authorization` | `Bearer <ADMIN_JWT_TOKEN>` | 是 |

#### 请求体 (Request Body)
| 字段名 | 类型 | 必填 | 描述 |
| :--- | :--- | :--- | :--- |
| `character_id` | `integer` | 是 | 目标角色 ID |
| `is_up` | `boolean` | 否 | 是否设为当前卡池的 UP 角色 (默认 `false`) |

#### cURL 示例
```bash
curl -X POST http://localhost:8080/api/admin/pool/push \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <ADMIN_JWT_TOKEN>" \
  -d '{
    "character_id": 11,
    "is_up": true
  }'
```

#### 成功响应 (200 OK)
```json
{
  "message": "character pushed into pool successfully",
  "data": {
    "id": 11,
    "name": "极光战姬·艾莉亚",
    "rarity": "S",
    "is_limited": true,
    "in_pool": true,
    "is_up": true,
    "entered_pool_at": "2026-10-06T13:00:00Z"
  }
}
```

#### 错误响应 (404 Not Found - 角色不存在)
```json
{
  "error": "unable to find the character"
}
```

#### 错误响应 (403 Forbidden)
```json
{
  "error": "permission denied"
}
```

---

### 4.3 批量加载预设卡池 (POST /api/admin/pool/load-presets)
- **接口路径**: `POST /api/admin/pool/load-presets`
- **权限要求**: 管理员 (Role: admin)
- **接口描述**: 从本地 JSON 预设文件批量读取角色配置并导入数据库与卡池。默认读取路径为 `presets/characters.json`。为保障系统安全，禁止跨目录读取（路径必须位于 `presets` 目录内）。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Content-Type` | `application/json` | 否 (有 Request Body 时必填) |
| `Authorization` | `Bearer <ADMIN_JWT_TOKEN>` | 是 |

#### 请求体 (Request Body，可选)
| 字段名 | 类型 | 必填 | 默认值 | 描述 |
| :--- | :--- | :--- | :--- | :--- |
| `file_path` | `string` | 否 | `"presets/characters.json"` | 预设 JSON 相对文件路径（必须位于 presets 目录内） |

#### cURL 示例
```bash
# 默认加载预设
curl -X POST http://localhost:8080/api/admin/pool/load-presets \
  -H "Authorization: Bearer <ADMIN_JWT_TOKEN>"

# 指定预设文件路径
curl -X POST http://localhost:8080/api/admin/pool/load-presets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <ADMIN_JWT_TOKEN>" \
  -d '{
    "file_path": "presets/characters.json"
  }'
```

#### 成功响应 (200 OK)
```json
{
  "message": "presets loaded successfully",
  "count": 10,
  "data": [
    {
      "id": 1,
      "name": "星渊猎手·卡莲",
      "rarity": "S",
      "is_limited": true,
      "in_pool": true,
      "is_up": true,
      "entered_pool_at": "2026-10-06T10:00:00Z"
    },
    {
      "id": 2,
      "name": "虚数神机·天枢",
      "rarity": "S",
      "is_limited": true,
      "in_pool": true,
      "is_up": false,
      "entered_pool_at": "2026-10-06T10:00:00Z"
    }
  ]
}
```

#### 错误响应 (400 Bad Request - 非法路径)
```json
{
  "error": "invalid file path: must be located inside presets directory"
}
```

---

### 4.4 动态热更新卡池配置 (PUT /api/admin/pool/config)
- **接口路径**: `PUT /api/admin/pool/config`
- **权限要求**: 管理员 (Role: admin)
- **接口描述**: 动态修改卡池全局概率与保底阈值配置。后端采用 `sync/atomic.Pointer` 实现无锁原子配置替换，零停机热重载，并自动触发：
  1. 通过 gRPC 服务端流（`ConfigService.SubscribeConfigUpdates`）将新配置秒级广播至所有分布式游戏服务节点。
  2. 通过 SSE 事件流广播 `PROB_UPDATE`，通知所有在线前端客户端刷新本地卡池展示。

#### 请求头 (Headers)
| 头部名称 | 取值 | 是否必填 |
| :--- | :--- | :--- |
| `Content-Type` | `application/json` | 是 |
| `Authorization` | `Bearer <ADMIN_JWT_TOKEN>` | 是 |

#### 请求体 (Request Body)
全部字段均为可选，未传入字段将保留现有配置：
| 字段名 | 类型 | 必填 | 约束 | 描述 |
| :--- | :--- | :--- | :--- | :--- |
| `base_rate_s` | `float64` | 否 | `0.0 <= rate <= 1.0` | S 档基础概率 |
| `base_rate_a` | `float64` | 否 | `0.0 <= rate <= 1.0` | A 档基础概率 |
| `base_rate_b` | `float64` | 否 | `0.0 <= rate <= 1.0` | B 档基础概率 |
| `soft_pity_start` | `int` | 否 | `soft_pity_start >= 0` 且 `< hard_pity_s` | S 档软保底起始抽数 |
| `soft_pity_inc` | `float64` | 否 | `0.0 <= inc <= 1.0` | 软保底每抽增加的 S 概率 |
| `hard_pity_s` | `int` | 否 | `hard_pity_s > 0` | S 档硬保底阈值 |
| `hard_pity_a` | `int` | 否 | `hard_pity_a > 0` | A 档硬保底阈值 |
| `max_limited_s` | `int` | 否 | `max_limited_s > 0` | 卡池内最多共存限定 S 角色数 |

#### cURL 示例
```bash
curl -X PUT http://localhost:8080/api/admin/pool/config \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <ADMIN_JWT_TOKEN>" \
  -d '{
    "base_rate_s": 0.012,
    "hard_pity_s": 75,
    "soft_pity_start": 60,
    "soft_pity_inc": 0.06
  }'
```

#### 成功响应 (200 OK)
```json
{
  "message": "卡池配置更新成功",
  "config": {
    "base_rate_s": 0.012,
    "base_rate_a": 0.08,
    "base_rate_b": 0.912,
    "soft_pity_start": 60,
    "soft_pity_inc": 0.06,
    "hard_pity_s": 75,
    "hard_pity_a": 10,
    "max_limited_s": 3
  }
}
```

#### 错误响应 (400 Bad Request - 参数非法)
```json
{
  "error": "配置参数校验失败：概率与保底数值不合法"
}
```

---

## 5. 数据模型定义 (Data Models)

数据结构严格对应后端 `model.go` 及相关实体：

### 5.1 User (用户实体)
| 字段名 | JSON Key | 类型 | 描述 |
| :--- | :--- | :--- | :--- |
| `ID` | `id` | `string` | 用户全局唯一 8 位字符 ID |
| `Nickname` | `nickname` | `string` | 用户昵称 |
| `Password` | `-` | `string` | 密码哈希 (JSON 序列化忽略) |
| `Bio` | `bio` | `string` | 个人简介 |
| `Role` | `role` | `string` | 角色权限：`"user"` 或 `"admin"` |
| `Email` | `email` | `string?` | 绑定的邮箱 (若无则为 null 或省略) |
| `GithubID` | `github_id` | `string?` | 绑定的 GitHub ID (若无则为 null 或省略) |
| `PitySCount` | `pity_s_count` | `int` | S 档保底计数 (自上次抽到 S 后的累计抽数) |
| `PityACount` | `pity_a_count` | `int` | A 档保底计数 (自上次抽到 A 后的累计抽数) |
| `CreatedAt` | `created_at` | `string (ISO 8601)` | 用户创建时间 |
| `UpdateAt` | `updated_at` | `string (ISO 8601)` | 用户更新时间 |

### 5.2 Character (角色实体)
| 字段名 | JSON Key | 类型 | 描述 |
| :--- | :--- | :--- | :--- |
| `ID` | `id` | `uint` | 角色唯一主键 ID |
| `Name` | `name` | `string` | 角色名称 |
| `Rarity` | `rarity` | `string` | 稀有度档位：`"S"`, `"A"`, `"B"` |
| `IsLimited` | `is_limited` | `boolean` | 是否为限定角色 |
| `IsInPool` | `in_pool` | `boolean` | 是否在当前卡池中 |
| `IsUp` | `is_up` | `boolean` | 是否为当前卡池的 UP 角色 |
| `EnteredPoolAt` | `entered_pool_at` | `string? (ISO 8601)` | 角色推入卡池的时间戳 |

### 5.3 UserCharacter (用户持有角色 / 背包实体)
| 字段名 | JSON Key | 类型 | 描述 |
| :--- | :--- | :--- | :--- |
| `ID` | `id` | `uint` | 背包记录唯一主键 ID |
| `UserID` | `user_id` | `string` | 所属用户 ID |
| `CharacterID` | `character_id` | `uint` | 关联的角色 ID |
| `Character` | `character` | `Character` | 角色详细信息实体 |
| `Rank` | `rank` | `int` | 角色命座阶数 (初次抽取为 0，每重复抽到一次加 1) |

### 5.4 GachaRecord (抽卡历史记录实体)
| 字段名 | JSON Key | 类型 | 描述 |
| :--- | :--- | :--- | :--- |
| `ID` | `id` | `uint` | 记录唯一主键 ID |
| `UserID` | `user_id` | `string` | 抽卡所属用户 ID |
| `CharacterID` | `character_id` | `uint` | 抽到的角色 ID |
| `CharacterName` | `character_name` | `string` | 抽到的角色名称 |
| `Rarity` | `rarity` | `string` | 抽到的角色稀有度 (`"S"`, `"A"`, `"B"`) |
| `IsFirstTime` | `is_first_time` | `boolean` | 是否为该用户首次抽取到该角色 |
| `PityCount` | `pity_count` | `int` | 本次出货时消耗的 S 档保底抽数 |
| `CreatedAt` | `created_at` | `string (ISO 8601)` | 抽卡时间戳 |

### 5.5 PoolConfig (卡池算法与概率配置实体)
| 字段名 | JSON Key | 类型 | 默认值 | 描述 |
| :--- | :--- | :--- | :--- | :--- |
| `BaseRateS` | `base_rate_s` | `float64` | `0.008` (0.8%) | S 档基础概率 |
| `BaseRateA` | `base_rate_a` | `float64` | `0.080` (8.0%) | A 档基础概率 |
| `BaseRateB` | `base_rate_b` | `float64` | `0.912` (91.2%) | B 档基础概率 |
| `SoftPityStart` | `soft_pity_start` | `int` | `65` | S 档软保底起始抽数 |
| `SoftPityInc` | `soft_pity_inc` | `float64` | `0.050` (5.0%) | 软保底每抽增加的 S 概率 |
| `HardPityS` | `hard_pity_s` | `int` | `80` | S 档硬保底抽数 (必出 S) |
| `HardPityA` | `hard_pity_a` | `int` | `10` | A 档硬保底抽数 (必出 A 或 S) |
| `MaxLimitedS` | `max_limited_s` | `int` | `3` | 卡池内共存限定 S 角色最大数量上限 (FIFO) |

### 5.6 DivinationRecord (星穹每日占卜记录实体)
| 字段名 | JSON Key | 类型 | 描述 |
| :--- | :--- | :--- | :--- |
| `ID` | `id` | `uint` | 主键自增 ID |
| `UserID` | `user_id` | `string` | 关联用户唯一标识 (索引) |
| `Date` | `date` | `string` | 占卜自然日格式 (YYYY-MM-DD，联合索引) |
| `Sign` | `sign` | `string` | 占卜星象签文 (如 "大吉·星神注视") |
| `Description` | `description` | `string` | 命途神谕文本 |
| `RewardAmount` | `reward_amount` | `int` | 星穹赠礼星琼数量 (如 160) |
| `CreatedAt` | `created_at` | `string (ISO 8601)` | 抽取时间戳 |

---

## 6. gRPC 配置流式分发服务 (gRPC Config Streaming Service)

抽卡模拟器内置 gRPC 服务端流（Server-side Streaming RPC），用于集群多节点场景下的概率与保底配置近实时同步分发。

### 6.1 服务与 Proto 定义 (`proto/config.proto`)
- **默认监听端口**: `:50051`
- **传输协议**: HTTP/2 + Protocol Buffers v3

```protobuf
syntax = "proto3";

package proto;
option go_package = "./proto";

message EmptyRequest {}

message PoolConfigMessage {
  double base_rate_s = 1;
  double base_rate_a = 2;
  double base_rate_b = 3;
  int32 soft_pity_start = 4;
  double soft_pity_inc = 5;
  int32 hard_pity_s = 6;
  int32 hard_pity_a = 7;
  int32 max_limited_s = 8;
  int64 updated_at = 9;
}

service ConfigService {
  rpc SubscribeConfigUpdates(EmptyRequest) returns (stream PoolConfigMessage);
}
```

### 6.2 流式生命周期与行为规范
1. **握手与快照直发**: 客户端发起 `SubscribeConfigUpdates` 连接建立后，服务端立即推送当前内存生效的完整 `PoolConfigMessage` 首帧快照。
2. **多播与事件广播**: 当管理员通过 `PUT /api/admin/pool/config` 成功更新配置时，`DefaultGRPCServer` 将新配置推入非阻塞订阅通道并广播给所有活动订阅流。
3. **客户端自动重连与原子热重载**:
   - `grpc_client.go` 提供内置后台客户端 `StartGRPCConfigClient`，支持断线指数补偿重连。
   - 收到更新流后，通过 `GlobalConfigAtomic.Store(&newCfg)` 无锁原子更新全局配置，无需停机或加互斥锁，零损耗无缝切入后续祈愿计算。

---

## 7. 服务解耦架构与运行模式 (Architecture Decoupling & Server Run Modes)

为了应对生产级大规模微服务拆分诉求，系统在保持单体一体化部署能力的同时，深度解耦为两大独立职责服务：**管理与认证服务 (Management Server)** 与 **游戏与抽卡服务 (Game Server)**。

### 7.1 架构拆分与职责划分

```mermaid
flowchart TD
    subgraph ClientLayer["客户端接入层"]
        WebUI["Web 赛博朋克前端 (web/index.html)"]
        CLICli["Terminal CLI 终端 (cmd/cli/main.go)"]
    end

    subgraph ManagementSvr["管理与认证服务 (Management Server) :8080"]
        AuthModule["账号与认证 (注册/密码登录/邮箱/GitHub)"]
        UserMeModule["个人信息维护 (/user/profile, /user/me)"]
        AdminModule["卡池管理与运营 (/admin/character, /pool/push, /pool/config)"]
        GRPCSvr["gRPC ConfigService Server (:50051)"]
        PrivKey["Ed25519 私钥 (jwt_private.pem)"]
        AuthModule --> PrivKey
        AdminModule --> GRPCSvr
    end

    subgraph GameSvr["游戏与抽卡服务 (Game Server) :8081"]
        PoolInfoModule["公开卡池信息 (/pool/info)"]
        GachaDrawModule["祈愿抽卡 (/gacha/draw)"]
        InventoryModule["角色仓库 (/gacha/inventory)"]
        StatsModule["战报与流水 (/gacha/stats, /gacha/history)"]
        GRPCClient["gRPC Config Client (订阅 :50051)"]
        PubKey["Ed25519 公钥 (jwt_public.pem)"]
        GachaDrawModule --> PubKey
        GRPCClient --> PoolInfoModule
    end

    subgraph SharedData["持久化与密钥共享"]
        SQLiteDB[("SQLite 数据持久层 (gacha.db)")]
        DiskPEM["持久化 PEM 密钥对\n(jwt_private.pem & jwt_public.pem)"]
    end

    ClientLayer -->|认证与管理请求| ManagementSvr
    ClientLayer -->|抽卡与仓库查询| GameSvr

    ManagementSvr --> SQLiteDB
    GameSvr --> SQLiteDB
    ManagementSvr -.->|gRPC 流式推送最新配置| GameSvr
    PrivKey -.-> DiskPEM
    PubKey -.-> DiskPEM
```

#### 职责矩阵：
| 模块/功能 | 管理服务 (Management Server) | 游戏服务 (Game Server) | 联合模式 (Combined Mode) |
| :--- | :---: | :---: | :---: |
| 用户注册 / 登录 (`/api/register`, `/api/login`) | ✅ 负责 | ❌ 404 Not Found | ✅ 负责 |
| 邮箱 / GitHub OAuth 登录 (`/api/auth/*`) | ✅ 负责 | ❌ 404 Not Found | ✅ 负责 |
| 个人资料 / 信息查询 (`/api/user/*`) | ✅ 负责 | ❌ 404 Not Found | ✅ 负责 |
| 管理员后台 (`/api/admin/*`) | ✅ 负责 | ❌ 404 Not Found | ✅ 负责 |
| SSE 实时通知流 (`/api/notifications`) | ✅ 负责 | ❌ 404 Not Found | ✅ 负责 |
| gRPC 配置广播服务端 (`:50051`) | ✅ 启动 | ❌ 不启动 | ✅ 启动 |
| 公开卡池信息公示 (`/api/pool/info`) | ❌ 404 Not Found | ✅ 负责 | ✅ 负责 |
| 祈愿抽卡 (`/api/gacha/draw`) | ❌ 404 Not Found | ✅ 负责 | ✅ 负责 |
| 角色仓库 (`/api/gacha/inventory`) | ❌ 404 Not Found | ✅ 负责 | ✅ 负责 |
| 抽卡战报与历史 (`/api/gacha/stats`, `/api/gacha/history`) | ❌ 404 Not Found | ✅ 负责 | ✅ 负责 |
| gRPC 配置订阅客户端 (`StartGRPCConfigClient`) | ❌ 不启动 | ✅ 启动连接 | ✅ 启动连接 |

### 7.2 命令行启动模式与参数

通过可执行文件支持的命令行参数，运维人员可灵活切换服务形态：

```bash
# 查看所有支持的标志
go run main.go --help
```

- `--mode`: 服务运行模式，可选值：
  - `all` (默认): 单体联合模式，同时挂载全部路由，默认监听 `:8080` (HTTP) 与 `:50051` (gRPC)。
  - `management`: 管理服务器模式，仅挂载认证与管理接口，默认监听 `:8080` (HTTP) 与 `:50051` (gRPC Server)。
  - `game`: 游戏服务器模式，仅挂载卡池公示与抽卡业务接口，默认监听 `:8081` (HTTP)，并自动以后台 gRPC 客户端长连 `localhost:50051` 同步卡池配置。
- `--port`: 可选，自定义当前 HTTP 服务的监听端口（例如 `--port=9090`）。

#### 快速启动示例：
```bash
# 场景一：单体模式（默认）
go run main.go --mode=all

# 场景二：解耦集群模式
# 终端 1 启动管理端：
go run main.go --mode=management --port=8080

# 终端 2 启动游戏端：
go run main.go --mode=game --port=8081
```

### 7.3 Ed25519 非对称密钥与跨服务免密鉴权

在服务彻底解耦后，微服务面临经典问题：**Game Server 如何在不调用 Management Server 鉴权接口、不共享私钥的前提下，安全解析与校验用户身份？**

系统基于 **Ed25519 (EdDSA) 非对称椭圆曲线数字签名算法** 实现了高安全、零 RPC 损耗的免密鉴权闭环：
1. **持久化密钥生成 (`jwt.go`)**：
   - 服务初次启动时，自动生成 256 位 Ed25519 密钥对，并以标准 PKCS#8 / PKIX PEM 格式持久化至磁盘文件 `jwt_private.pem` 和 `jwt_public.pem`。
   - 随后的所有实例启动或子进程将直接从磁盘装载同一组权威密钥，确保重启或跨进程状态一致。
2. **私钥隔离与签名 (`Management Server`)**：
   - 只有 Management Server 持有并使用 `privateKey` 执行 `GenerateToken` 签名操作。
3. **公钥分发与无状态本地验签 (`Game Server`)**：
   - Game Server 仅需持有一份只读的 `jwt_public.pem`（或共享同目录读取）。
   - 在处理每一次 `/api/gacha/draw` 或 `/api/gacha/inventory` 时，通过 `jwt.ParseWithClaims(token, ..., publicKey)` 在本地微秒级完成签名有效性、过期时间及角色 Claims 的数学证明，**无需向 Management Server 发起任何同步 RPC 验证请求**，杜绝了鉴权网络风暴与性能瓶颈。

---

## 8. CLI 终端客户端使用指南 (Terminal CLI Client Guide)

抽卡模拟器内置全功能、交互式纯 Go 命令行客户端，位于 `cmd/cli/main.go`。该客户端专为极客开发者与自动化脚本打造，支持全彩 ANSI 炫酷终端渲染与免密会话缓存。

### 8.1 编译与快速上手

```bash
# 编译独立二进制
go build -o gacha-cli ./cmd/cli

# 查看帮助信息
./gacha-cli --help
```

#### 支持参数：
| 参数名 | 默认值 | 描述 |
| :--- | :--- | :--- |
| `--server` | `http://localhost:8080` | 管理服务器 URL (负责登录注册) |
| `--game-server` | 与 `--server` 保持一致 | 游戏服务器 URL (若解耦部署可指定为 `http://localhost:8081`) |
| `--cmd` | 空 (进入交互 REPL) | 非交互式直接指令执行模式 |
| `--id` | 空 | 配合 `--cmd=login` 使用的用户 ID |
| `--password` | 空 | 配合 `--cmd=login` / `--cmd=register` 使用的密码 |
| `--nickname` | 空 | 配合 `--cmd=register` 使用的昵称 |
| `--bio` | 空 | 配合 `--cmd=register` 使用的简介 |

### 8.2 交互式 REPL 菜单模式

直接运行 `./gacha-cli`（或在解耦模式下运行 `./gacha-cli --server http://localhost:8080 --game-server http://localhost:8081`），将进入交互式主菜单：

```text
=================================================================
         ✦  ASTRAL GACHA SIMULATOR - TERMINAL CLI  ✦           
=================================================================
 [用户]: 幸运开拓者 (73b6ae34) | [S保底]: 42/80 | [A保底]: 4/10
 [管理服务器]: http://localhost:8080 | [游戏服务器]: http://localhost:8081
-----------------------------------------------------------------
 1.  登录 (Login)
 2.  注册 (Register)
 3.  查看卡池信息 (Pool Info)
 4.  单抽 (Draw 1)
 5.  十连抽 (Draw 10)
 6.  查看角色仓库 (Inventory)
 7.  抽卡战报统计 (Stats)
 8.  查看抽卡历史流水 (History)
 9.  清空抽卡历史与保底 (Clear History)
 10. 登出当前账号 (Logout)
 11. 星穹每日占卜 (Daily Divination)
 12. 蒙特卡洛抽卡测算 (Monte Carlo Sim)
 0.  退出程序 (Exit)
-----------------------------------------------------------------
请选择操作 [0-12]: 
```

### 8.3 非交互式直接指令模式

适合通过终端管道或脚本自动化批量调用：

```bash
# 1. 命令行直接注册并自动登录
./gacha-cli --cmd register --nickname "auto_bot" --password "botpass123"

# 2. 命令行直接登录
./gacha-cli --cmd login --id "auto_bot" --password "botpass123"

# 3. 查看卡池信息
./gacha-cli --cmd pool

# 4. 执行十连抽
./gacha-cli --cmd draw10

# 5. 查看角色背包清单
./gacha-cli --cmd inventory

# 6. 查看出金分析与歪卡战报
./gacha-cli --cmd stats

# 7. 查看历史抽卡流水
./gacha-cli --cmd history

# 8. 每日星穹占卜获取签文与星琼
./gacha-cli --cmd divination

# 9. 蒙特卡洛极速概率测算 (默认 1000 抽，支持 --pulls 调整)
./gacha-cli --cmd sim --pulls 5000
```

### 8.4 本地凭证持久化与高亮输出

1. **会话缓存 (`.gacha_cli_token`)**：
   - 登录成功后，Token 与用户资料将自动写入本地文件 `.gacha_cli_token`；
   - 每次 CLI 启动时自动读取并校验会话，无需每次重复输入密码；
   - 执行登出或清空历史时自动安全注销。
2. **ANSI 终端高品质调色**：
   - **金光耀目 (Gold / Yellow)**：S 级角色与限定 UP 尊贵标识；
   - **紫电流转 (Purple / Magenta)**：A 级角色与十连保底；
   - **青空如洗 (Cyan / Blue)**：B 级武器与量产装备；
   - **鲜绿与正红 (Green / Red)**：成功与告警提示。


