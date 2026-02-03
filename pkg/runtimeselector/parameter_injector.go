package runtimeselector

import (
	"fmt"
	"strconv"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// VLLMOptimizationProfiles defines engine arguments for each optimization policy for vLLM.
var VLLMOptimizationProfiles = map[v1beta1.OptimizationPolicy][]string{
	v1beta1.LatencyOptimized: {
		"--enable-chunked-prefill",
		"--max-num-batched-tokens=2048",
	},
	v1beta1.ThroughputOptimized: {
		"--disable-log-requests",
		"--max-num-seqs=256",
		"--gpu-memory-utilization=0.95",
		"--enable-prefix-caching",
	},
	v1beta1.CostOptimized: {
		"--max-num-seqs=128",
		"--gpu-memory-utilization=0.85",
	},
	v1beta1.Balanced: {}, // Use vLLM defaults
}

// SGLangOptimizationProfiles defines engine arguments for each optimization policy for SGLang.
var SGLangOptimizationProfiles = map[v1beta1.OptimizationPolicy][]string{
	v1beta1.LatencyOptimized: {
		"--chunked-prefill-size=2048",
		"--schedule-policy=fcfs",
	},
	v1beta1.ThroughputOptimized: {
		"--schedule-policy=lpm",
		"--max-running-requests=256",
		"--mem-fraction-static=0.95",
	},
	v1beta1.CostOptimized: {
		"--max-running-requests=128",
		"--mem-fraction-static=0.85",
	},
	v1beta1.Balanced: {}, // Use SGLang defaults
}

// TGIOptimizationProfiles defines engine arguments for each optimization policy for TGI.
var TGIOptimizationProfiles = map[v1beta1.OptimizationPolicy][]string{
	v1beta1.LatencyOptimized: {
		"--max-concurrent-requests=32",
		"--max-batch-prefill-tokens=2048",
	},
	v1beta1.ThroughputOptimized: {
		"--max-concurrent-requests=256",
		"--max-batch-total-tokens=32768",
	},
	v1beta1.CostOptimized: {
		"--max-concurrent-requests=64",
	},
	v1beta1.Balanced: {}, // Use TGI defaults
}

// DefaultParameterInjector implements ParameterInjector with standard optimization profiles.
type DefaultParameterInjector struct {
	config *Config
}

// NewDefaultParameterInjector creates a new DefaultParameterInjector.
func NewDefaultParameterInjector(config *Config) ParameterInjector {
	return &DefaultParameterInjector{
		config: config,
	}
}

// InjectParameters modifies the runtime spec to include optimization parameters.
func (p *DefaultParameterInjector) InjectParameters(
	runtime *v1beta1.ServingRuntimeSpec,
	requirements *v1beta1.ServiceRequirements,
) error {
	if requirements == nil || runtime == nil {
		return nil
	}

	// Detect engine type from runtime
	engineType := p.detectEngineType(runtime)
	if engineType == "" {
		// Cannot detect engine type, skip injection
		return nil
	}

	// Get optimization policy (default to Balanced)
	policy := requirements.OptimizationPolicy
	if policy == "" {
		policy = v1beta1.Balanced
	}

	// Get the optimization arguments
	args := p.GetOptimizationArgs(engineType, policy, requirements)
	if len(args) == 0 {
		return nil
	}

	// Inject arguments into all containers in the runtime
	for i := range runtime.Containers {
		container := &runtime.Containers[i]
		container.Args = append(container.Args, args...)
	}

	return nil
}

// GetOptimizationArgs returns the engine arguments for a specific optimization policy and engine type.
func (p *DefaultParameterInjector) GetOptimizationArgs(
	engineType string,
	policy v1beta1.OptimizationPolicy,
	requirements *v1beta1.ServiceRequirements,
) []string {
	var args []string

	// Get profile-specific arguments based on engine type
	switch engineType {
	case EngineTypeVLLM:
		if profileArgs, ok := VLLMOptimizationProfiles[policy]; ok {
			args = append(args, profileArgs...)
		}
		// Add requirement-specific arguments for vLLM
		args = append(args, p.getVLLMRequirementArgs(requirements)...)

	case EngineTypeSGLang:
		if profileArgs, ok := SGLangOptimizationProfiles[policy]; ok {
			args = append(args, profileArgs...)
		}
		// Add requirement-specific arguments for SGLang
		args = append(args, p.getSGLangRequirementArgs(requirements)...)

	case EngineTypeTGI:
		if profileArgs, ok := TGIOptimizationProfiles[policy]; ok {
			args = append(args, profileArgs...)
		}
		// Add requirement-specific arguments for TGI
		args = append(args, p.getTGIRequirementArgs(requirements)...)
	}

	return args
}

// detectEngineType tries to detect the engine type from the runtime spec.
func (p *DefaultParameterInjector) detectEngineType(runtime *v1beta1.ServingRuntimeSpec) string {
	// Check supported model formats for hints
	for _, format := range runtime.SupportedModelFormats {
		if format.ModelFormat == nil {
			continue
		}
		name := format.ModelFormat.Name
		switch name {
		case "vLLM", "vllm":
			return EngineTypeVLLM
		case "srt", "sglang", "SGLang":
			return EngineTypeSGLang
		case "tgi", "TGI", "text-generation-inference":
			return EngineTypeTGI
		}
	}

	// Check container images for hints
	for _, container := range runtime.Containers {
		image := container.Image
		if containsAny(image, "vllm") {
			return EngineTypeVLLM
		}
		if containsAny(image, "sglang", "srt") {
			return EngineTypeSGLang
		}
		if containsAny(image, "tgi", "text-generation-inference") {
			return EngineTypeTGI
		}
	}

	return ""
}

// getVLLMRequirementArgs returns vLLM-specific arguments based on requirements.
func (p *DefaultParameterInjector) getVLLMRequirementArgs(requirements *v1beta1.ServiceRequirements) []string {
	var args []string

	if requirements == nil {
		return args
	}

	// Add context length argument
	if requirements.MaxContextLength != nil && *requirements.MaxContextLength > 0 {
		args = append(args, fmt.Sprintf("--max-model-len=%d", *requirements.MaxContextLength))
	}

	// Add concurrency argument
	if requirements.MaxConcurrency != nil && *requirements.MaxConcurrency > 0 {
		args = append(args, fmt.Sprintf("--max-num-seqs=%d", *requirements.MaxConcurrency))
	}

	return args
}

// getSGLangRequirementArgs returns SGLang-specific arguments based on requirements.
func (p *DefaultParameterInjector) getSGLangRequirementArgs(requirements *v1beta1.ServiceRequirements) []string {
	var args []string

	if requirements == nil {
		return args
	}

	// Add context length argument
	if requirements.MaxContextLength != nil && *requirements.MaxContextLength > 0 {
		args = append(args, fmt.Sprintf("--context-length=%d", *requirements.MaxContextLength))
	}

	// Add concurrency argument
	if requirements.MaxConcurrency != nil && *requirements.MaxConcurrency > 0 {
		args = append(args, fmt.Sprintf("--max-running-requests=%d", *requirements.MaxConcurrency))
	}

	return args
}

// getTGIRequirementArgs returns TGI-specific arguments based on requirements.
func (p *DefaultParameterInjector) getTGIRequirementArgs(requirements *v1beta1.ServiceRequirements) []string {
	var args []string

	if requirements == nil {
		return args
	}

	// Add context length argument
	if requirements.MaxContextLength != nil && *requirements.MaxContextLength > 0 {
		args = append(args, fmt.Sprintf("--max-input-length=%d", *requirements.MaxContextLength))
	}

	// Add concurrency argument
	if requirements.MaxConcurrency != nil && *requirements.MaxConcurrency > 0 {
		args = append(args, fmt.Sprintf("--max-concurrent-requests=%d", *requirements.MaxConcurrency))
	}

	return args
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// Helper to convert int64 to string (used internally)
func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}
