#!/usr/bin/env python3
"""Send a synthetic signed webhook to the local demo (never to Meta)."""
import hashlib
import hmac
import json
import sys
import time
import urllib.request
import uuid

body = json.dumps({
    "object": "whatsapp_business_account",
    "entry": [{"changes": [{"value": {
        "metadata": {"phone_number_id": "demo-phone"},
        "messages": [{"id": "demo-" + str(uuid.uuid4()), "from": "399000000001",
                      "timestamp": str(int(time.time())), "type": "text",
                      "text": {"body": sys.argv[1] if len(sys.argv) > 1 else "Quando ci alleniamo?"}}]
    }}]}]
}).encode()
signature = hmac.new(b"demo-secret-not-for-production", body, hashlib.sha256).hexdigest()
request = urllib.request.Request("http://127.0.0.1:8080/webhook", data=body, headers={
    "Content-Type": "application/json", "X-Hub-Signature-256": "sha256=" + signature
})
with urllib.request.urlopen(request, timeout=10) as response:
    print("Webhook demo accettato:", response.status)
print("Controlla il log reply_accepted. La demo non mostra risposte e non invia messaggi reali.")
