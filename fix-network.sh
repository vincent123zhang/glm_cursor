#!/bin/bash
# 修复代理/VPN TUN fake-ip 残留路由
# 使用方法: bash fix-network.sh

set -u

TARGET_DOMAIN="${1:-chat.glm.ai}"

delete_route() {
  local label="$1"
  local net="$2"
  local mask="$3"

  if sudo route -n delete -net "$net" -netmask "$mask" 198.18.0.1 >/dev/null 2>&1; then
    echo "  ✅ 已删除 $label"
  elif sudo route -n delete -net "$net" -netmask "$mask" >/dev/null 2>&1; then
    echo "  ✅ 已删除 $label"
  else
    echo "  ⏭  $label 不存在，跳过"
  fi
}

echo "🔧 清理 VPN/代理 TUN fake-ip 残留路由..."
delete_route "1.0.0.0/8"     "1.0.0.0"     "255.0.0.0"
delete_route "2.0.0.0/7"     "2.0.0.0"     "254.0.0.0"
delete_route "4.0.0.0/6"     "4.0.0.0"     "252.0.0.0"
delete_route "8.0.0.0/5"     "8.0.0.0"     "248.0.0.0"
delete_route "16.0.0.0/4"    "16.0.0.0"    "240.0.0.0"
delete_route "32.0.0.0/3"    "32.0.0.0"    "224.0.0.0"
delete_route "64.0.0.0/2"    "64.0.0.0"    "192.0.0.0"
delete_route "128.0.0.0/1"   "128.0.0.0"   "128.0.0.0"

echo ""
echo "🔧 关闭常见系统代理..."
while IFS= read -r service; do
  [ -z "$service" ] && continue
  networksetup -setwebproxystate "$service" off >/dev/null 2>&1
  networksetup -setsecurewebproxystate "$service" off >/dev/null 2>&1
  networksetup -setsocksfirewallproxystate "$service" off >/dev/null 2>&1
  networksetup -setautoproxystate "$service" off >/dev/null 2>&1
done < <(networksetup -listallnetworkservices 2>/dev/null | sed '1d')
echo "  ✅ 已尝试关闭 HTTP/HTTPS/SOCKS/PAC 代理"

echo ""
echo "🔧 DNS 恢复为自动获取..."
while IFS= read -r service; do
  [ -z "$service" ] && continue
  sudo networksetup -setdnsservers "$service" empty >/dev/null 2>&1
done < <(networksetup -listallnetworkservices 2>/dev/null | sed '1d')
echo "  ✅ 已恢复 DNS 自动获取"

echo ""
echo "🔧 刷新 DNS 缓存..."
sudo dscacheutil -flushcache
sudo killall -HUP mDNSResponder 2>/dev/null
echo "  ✅ DNS 缓存已清空"

echo ""
echo "🔧 验证 $TARGET_DOMAIN ..."
RESULT=$(dscacheutil -q host -a name "$TARGET_DOMAIN" 2>/dev/null | awk '/ip_address/ {print $2; exit}')
ROUTE=$(route -n get "$TARGET_DOMAIN" 2>/dev/null | awk '/interface:/ {print $2; exit}')

if [ -z "$RESULT" ]; then
  echo "  ❌ $TARGET_DOMAIN 没有解析出 IP"
  echo "  建议：断开/重连 Wi-Fi，或检查公司/办公网络是否下发 DNS。"
elif echo "$RESULT" | grep -q "^198\.18\."; then
  echo "  ❌ $TARGET_DOMAIN 仍解析到 fake-ip: $RESULT"
  echo "  建议：彻底退出 Clash/Mihomo/Surge/sing-box 等代理客户端，关闭 TUN/fake-ip，再重启电脑。"
elif [ "$ROUTE" = "utun8" ] || echo "$ROUTE" | grep -q "^utun"; then
  echo "  ❌ $TARGET_DOMAIN ($RESULT) 仍走虚拟网卡: $ROUTE"
  echo "  建议：关闭 VPN/代理的 TUN 模式，或重启电脑清除 utun 残留。"
else
  echo "  ✅ $TARGET_DOMAIN → $RESULT，当前出口网卡: ${ROUTE:-未知}"
  echo "  可以重新打开浏览器访问：https://$TARGET_DOMAIN/"
fi
