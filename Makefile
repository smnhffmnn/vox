.PHONY: build app clean

build:
	go build -tags tray -o vox .

app: build
	rm -rf Vox.app
	mkdir -p Vox.app/Contents/MacOS
	cp vox Vox.app/Contents/MacOS/vox
	cp packaging/Info.plist Vox.app/Contents/Info.plist
	cp packaging/vox-launcher Vox.app/Contents/MacOS/vox-launcher
	chmod +x Vox.app/Contents/MacOS/vox-launcher
	@echo "Built Vox.app"

clean:
	rm -f vox
	rm -rf Vox.app
