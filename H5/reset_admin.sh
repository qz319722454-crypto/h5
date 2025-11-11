#!/bin/bash

echo "方法1: 使用后端 reset-admin 端点（如果已添加）..."
echo "重启后端容器..."
docker-compose restart backend
sleep 8

echo "调用重置端点..."
RESULT=$(curl -s -X POST http://127.0.0.1:8080/admin/reset-admin \
  -H "Content-Type: application/json" \
  -d '{}')

if echo "$RESULT" | grep -q "message"; then
    echo "成功！$RESULT"
    echo "现在可以使用 admin/admin 登录"
else
    echo "端点不存在或失败，使用方法2..."
    echo ""
    echo "方法2: 直接生成哈希并更新..."
    
    # 生成哈希
    HASH=$(docker exec h5-backend sh -c 'cat > /tmp/gen.go << "EOF"
package main
import ("fmt"; "golang.org/x/crypto/bcrypt")
func main() { h, _ := bcrypt.GenerateFromPassword([]byte("admin"), 10); fmt.Print(string(h)) }
EOF
go run /tmp/gen.go' 2>/dev/null)
    
    if [ -n "$HASH" ] && [ ${#HASH} -gt 50 ]; then
        echo "生成的哈希: ${HASH:0:30}..."
        docker exec -i h5-mysql mysql -uroot -prootpass customer_service_db << EOF
UPDATE customer_services SET password = '$HASH', is_admin = 1 WHERE name = 'admin';
SELECT name, LEFT(password, 30) as hash_preview, is_admin FROM customer_services WHERE name = 'admin';
EOF
        echo "完成！现在可以使用 admin/admin 登录"
    else
        echo "错误: 无法生成哈希"
        echo "请手动运行: docker exec h5-backend sh -c 'cd /app && go run - <<EOF"
        echo "package main"
        echo "import (\"fmt\"; \"golang.org/x/crypto/bcrypt\")"
        echo "func main() { h, _ := bcrypt.GenerateFromPassword([]byte(\"admin\"), 10); fmt.Print(string(h)) }"
        echo "EOF'"
    fi
fi
