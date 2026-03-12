#!/bin/bash
_curl_output=$(curl -is -X POST http://192.168.1.2:1969/mcp \
  -H "Content-Type: application/json" \
  -d '{"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"curl-test","version":"1.0.0"}},"jsonrpc":"2.0","id":0}')

_mcp_sid=$(grep -oP '^Mcp-Session-Id: \K[^$]+' <<< $_curl_output)

curl -s -X POST http://192.168.1.2:1969/mcp \
  -H "" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_system_status","arguments":{}},"id":2}'

