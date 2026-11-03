#!/usr/bin/env python3
"""一个假的 JumpServer：只答 jms_status / jms_doctor 用到的那几条路由。

为什么它在仓里：真堡垒机**分不出身份是从哪儿来的**（profile 和用户列表的
第一行在真机上恰好一致）⇒ 一个「故意让两条路由说法不同」的假服务，是唯一
能判别身份来源的办法。

用法：
    python3 script/fake-bastion.py <模式> <端口>
      diff    /users/profile/ 说 probe_user，而 /users/users/ 的第一行是
              admin ⇒ 答 admin 的是从列表拿的身份，答 probe_user 的是从
              profile 拿的身份。
      broken  /users/profile/ 回 HTTP 500 ⇒ 身份建立不起来，服务**不该**
              报任何身份。

⚠️ 它只监听 127.0.0.1：这是一个测试替身，不是给真实部署用的。
"""
import json
import sys
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

MODE = sys.argv[1]
PORT = int(sys.argv[2])

USERS_LIST = {
    "count": 2,
    "results": [
        {"id": "u-admin", "username": "admin", "name": "Administrator",
         "is_superuser": True},
        {"id": "u-other", "username": "zz_mcp_probe", "name": "Probe",
         "is_superuser": False},
    ],
}
PROFILE_OK = {"id": "u-probe", "username": "probe_user", "name": "Probe User",
              "is_superuser": False}
ASSETS = {"count": 0, "results": []}


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def do_GET(self):
        path = self.path.split("?")[0]
        if path == "/api/v1/users/profile/":
            if MODE == "broken":
                self._send(500, {"detail": "boom"})
            else:
                self._send(200, PROFILE_OK)
        elif path == "/api/v1/users/users/":
            self._send(200, USERS_LIST)
        elif path == "/api/v1/assets/assets/":
            self._send(200, ASSETS)
        elif path == "/api/v1/permissions/permissions/":
            self._send(200, {"count": 0, "results": []})
        else:
            self._send(200, {"count": 0, "results": []})

    def _send(self, code, body):
        raw = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)


srv = HTTPServer(("127.0.0.1", PORT), H)
t = threading.Thread(target=srv.serve_forever, daemon=True)
t.start()
print("listening", PORT, MODE, flush=True)
try:
    t.join()
except KeyboardInterrupt:
    pass
