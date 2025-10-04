Enable Metal in go-llama.cpp (Easiest, No Runtime Switch)
This keeps your existing code mostly unchanged while unlocking GPU acceleration on Apple Silicon. Llama.cpp's Metal backend is enabled by default on macOS builds.

Build llama.cpp with Metal:

Clone and build the underlying llama.cpp lib:
textgit clone <https://github.com/ggerganov/llama.cpp>
cd llama.cpp
make LLAMA_METAL=1  # Explicitly enables Metal; it's auto-detected on Apple Silicon

This compiles libllama.a (or shared lib) with Metal support.

Update go-llama.cpp:

In your Go module, ensure you're using the latest github.com/go-skynet/go-llama.cpp (v0.3+ for best Metal compatibility).
When initializing the model in Go, set the backend to Metal:
gopackage main

import (
    "fmt"
    "log"

    llm "github.com/go-skynet/go-llama.cpp"
)

func main() {
    // Load your GGUF model
    model, err := llm.New(
        "./my-model.gguf",
        llm.SetContext(2048),  // Adjust based on your needs
        llm.SetSeed(42),
        llm.SetF16(true),  // Use float16 for Apple Silicon efficiency
    )
    if err != nil {
        log.Fatal(err)
    }
    defer model.Free()

    // Enable Metal backend (via context params or env var)
    // Set env var before init: os.Setenv("LLAMA_METAL", "1")
    // Or pass via New options if supported in your version
    ctx := llm.NewContextParams()
    ctx.NGpuLayers = 999  // Offload all layers to GPU (Metal)
    ctx.UseMlock = false  // Avoid locking on unified memory
    session, err := model.CreateSession(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer session.Free()

    // Inference example
    prompt := "Hello, world!"
    output, err := session.Predict(prompt, llm.SetTemp(0.7), llm.SetTopP(0.9))
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(output)
}

Build your Go app on Apple Silicon: go build -tags metal (if tags are needed; check your go-llama.cpp version docs).
Test: Run on your Mac; it should auto-use the GPU. Monitor with htop or Activity Monitor (look for "Metal" in GPU usage).

Performance Tip: On an M3 Max, this can hit 50-100 tokens/sec for 7B models, depending on quantization. Benchmark with llama-bench from llama.cpp.
