module github.com/thanhhaudev/kizunax-plugin-cc

go 1.21

require (
	github.com/tetratelabs/wazero v1.8.0 // indirect
	github.com/thanhhaudev/phpsyms v0.2.1 // indirect
)

require github.com/thanhhaudev/llmreviewkit v1.5.0

replace github.com/thanhhaudev/llmreviewkit => ../llmreviewkit

replace github.com/thanhhaudev/phpsyms => ../phpsyms
