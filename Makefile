.PHONY: build clean

build:
		@echo "Configuring and building qjs..."
		cd quickjs && \
		rm -rf build && \
		cmake -B build \
				-DQJS_BUILD_LIBC=ON \
				-DQJS_BUILD_CLI_WITH_MIMALLOC=OFF \
				-DCMAKE_TOOLCHAIN_FILE=/opt/wasi-sdk/share/cmake/wasi-sdk.cmake \
				-DCMAKE_PROJECT_INCLUDE=../qjsextra/qjsextra.cmake
		@echo "Building qjs target..."
		make -C quickjs/build qjsextra -j$(nproc)
		@echo "Copying build/qjsextra to top-level as qjsextra.wasm..."
		cp quickjs/build/qjsextra qjsextra.wasm

clean:
	@echo "Cleaning build directory..."
	cd quickjs && rm -rf build

test:
	./test.sh

lint:
	golangci-lint run
