.PHONY: build build-all router-arm64 deploy test clean

build:
	go build -ldflags="-s -w" -o bin/rsi ./cmd/rsi

test:
	go test -v ./...

router-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/rsi-linux-arm64 ./cmd/rsi

deploy: router-arm64
	ssh RSI@192.168.50.1 "cat > /jffs/mihomo/rsi && chmod +x /jffs/mihomo/rsi && ln -sf /jffs/mihomo/rsi /koolshare/bin/rsi" < bin/rsi-linux-arm64
	@echo "✅ Deployed rsi to router successfully!"

clean:
	rm -rf bin/ dist/
