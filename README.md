# 🎮 Tetris Battle — VS Code 連線對戰俄羅斯方塊

> 等待 Claude Code 的時候，和朋友來一場俄羅斯方塊對戰！

消行攻擊模式：消除的行數會變成垃圾行送到對手的板子上，先死的人輸！

```
┌─────────┐          ┌──────────┐          ┌─────────┐
│ VS Code │◄────────►│ Go Server│◄────────►│ VS Code │
│ Player A│ WebSocket│ (雲端)   │ WebSocket│ Player B│
└─────────┘          └──────────┘          └─────────┘
```

---

## 架構

- **Server**: Go + Gorilla WebSocket，處理房間配對與狀態中繼
- **Client**: VS Code Extension + HTML5 Canvas WebView

---

## 🚀 部署伺服器 (3 種方式)

### 方式一：Fly.io（推薦，免費方案可用）

```bash
cd server

# 安裝 Fly CLI
curl -L https://fly.io/install.sh | sh

# 登入
fly auth login

# 部署（第一次會建立 app）
fly launch --name tetris-battle --region nrt

# 之後更新
fly deploy
```

部署完成後，你的 WebSocket 地址是：
```
wss://tetris-battle.fly.dev/ws
```

### 方式二：Railway / Render

1. 把 `server/` 推到 GitHub
2. 到 [Railway](https://railway.app) 或 [Render](https://render.com) 建立新專案
3. 連結 GitHub repo，設定 root directory 為 `server`
4. Railway 會自動偵測 Go 並部署

### 方式三：自己的 VPS

```bash
cd server
go build -o tetris-server .
PORT=8080 ./tetris-server

# 用 nginx 反向代理加上 SSL：
# location /ws {
#     proxy_pass http://127.0.0.1:8080/ws;
#     proxy_http_version 1.1;
#     proxy_set_header Upgrade $http_upgrade;
#     proxy_set_header Connection "upgrade";
# }
```

---

## 🎮 安裝 VS Code 擴充套件

```bash
cp -r vscode-ext ~/.vscode/extensions/tetris-battle
```

重啟 VS Code 後，到 Settings 搜尋 `tetrisBattle`，設定：

| 設定 | 說明 |
|------|------|
| `tetrisBattle.serverUrl` | 你部署的伺服器地址，例如 `wss://tetris-battle.fly.dev/ws` |
| `tetrisBattle.playerName` | 你的遊戲暱稱 |

按 `Ctrl+Shift+T`（Mac: `Cmd+Shift+T`）開始！

---

## 🕹️ 遊戲玩法

### 連線方式
- **隨機配對**：點擊「隨機配對」，系統自動幫你找對手
- **私人房間**：建立房間取得代碼，分享給朋友加入

### 操作
| 按鍵 | 功能 |
|------|------|
| `←` `→` | 左右移動 |
| `↑` | 旋轉 |
| `↓` | 加速下落 |
| `Space` | 硬降 |
| `C` | Hold 暫存 |

### 攻擊規則
| 消除行數 | 送出垃圾行 |
|----------|-----------|
| 1 行 (Single) | 0 行 |
| 2 行 (Double) | 1 行 |
| 3 行 (Triple) | 2 行 |
| 4 行 (Tetris) | 3 行 |

- 垃圾行會在你**放置下一個方塊後**從底部升起
- 垃圾行有一個隨機空隙，可以被消除
- 如果你有待接收的垃圾行，消行可以先**抵銷**一部分

---

## 專案結構

```
tetris-battle/
├── server/
│   ├── main.go          # WebSocket 伺服器
│   ├── go.mod
│   ├── Dockerfile       # Docker 部署用
│   └── fly.toml         # Fly.io 部署設定
├── vscode-ext/
│   ├── package.json     # VS Code 擴充套件設定
│   ├── extension.js     # 擴充套件入口
│   └── media/
│       └── battle.html  # 遊戲本體
└── README.md
```

---

## 通訊協定

Client ↔ Server 使用 JSON over WebSocket：

```jsonc
// 加入遊戲
{"type": "join", "data": {"name": "Alice", "room_id": "ABC123"}}

// 同步板面 (每 200ms)
{"type": "board_state", "data": {"grid": [...], "score": 1200, "lines": 8, "level": 2}}

// 送出攻擊
{"type": "attack", "data": {"lines": 2}}

// 遊戲結束
{"type": "game_over", "data": {}}
```

---

*在 Claude Code 努力寫程式的同時，你正在努力消滅對手 🧱⚔️*
