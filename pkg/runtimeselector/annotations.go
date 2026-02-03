package runtimeselector

import (
	"strconv"
	"strings"
)

// Runtime capability annotation keys.
// These annotations are used on ServingRuntime and ClusterServingRuntime resources
// to declare their performance capabilities for workload-aware runtime selection.
const (
	// AnnotationMaxContextLength specifies the maximum context length supported by the runtime.
	// Value: integer (number of tokens)
	// Example: "131072"
	AnnotationMaxContextLength = "ome.io/max-context-length"

	// AnnotationOptimizationProfiles specifies the optimization profiles supported by the runtime.
	// Value: comma-separated list of profiles
	// Example: "LatencyOptimized,ThroughputOptimized,Balanced"
	AnnotationOptimizationProfiles = "ome.io/optimization-profiles"

	// AnnotationMaxThroughput specifies the maximum throughput in tokens/second.
	// Value: integer (tokens per second)
	// Example: "500"
	AnnotationMaxThroughput = "ome.io/max-throughput"

	// AnnotationTypicalP99Latency specifies the typical P99 latency in milliseconds.
	// Value: integer (milliseconds)
	// Example: "50"
	AnnotationTypicalP99Latency = "ome.io/typical-p99-latency"

	// AnnotationMaxConcurrency specifies the maximum concurrent requests supported.
	// Value: integer (number of concurrent requests)
	// Example: "100"
	AnnotationMaxConcurrency = "ome.io/max-concurrency"

	// AnnotationCostTier specifies the cost tier of the runtime.
	// Value: "low", "medium", or "high"
	// Example: "low"
	AnnotationCostTier = "ome.io/cost-tier"

	// AnnotationGPUMemoryRequired specifies the GPU memory required.
	// Value: Kubernetes quantity string
	// Example: "40Gi"
	AnnotationGPUMemoryRequired = "ome.io/gpu-memory-required"

	// AnnotationEngineType specifies the inference engine type.
	// Value: "vllm", "sglang", "triton", "tgi"
	// Example: "vllm"
	AnnotationEngineType = "ome.io/engine-type"

	// AnnotationEngineVersion specifies the inference engine version.
	// Value: semantic version string
	// Example: "0.6.0"
	AnnotationEngineVersion = "ome.io/engine-version"
)

// Cost tier values
const (
	CostTierLow    = "low"
	CostTierMedium = "medium"
	CostTierHigh   = "high"
)

// Engine type values
const (
	EngineTypeVLLM   = "vllm"
	EngineTypeSGLang = "sglang"
	EngineTypeTriton = "triton"
	EngineTypeTGI    = "tgi"
)

// RuntimeCapabilities holds parsed capability values from runtime annotations.
type RuntimeCapabilities struct {
	// MaxContextLength is the maximum supported context length in tokens.
	MaxContextLength int64

	// OptimizationProfiles is the list of supported optimization profiles.
	OptimizationProfiles []string

	// MaxThroughput is the maximum throughput in tokens/second.
	MaxThroughput int64

	// TypicalP99Latency is the typical P99 latency in milliseconds.
	TypicalP99Latency int64

	// MaxConcurrency is the maximum number of concurrent requests.
	MaxConcurrency int64

	// CostTier is the cost classification (low, medium, high).
	CostTier string

	// EngineType is the inference engine type (vllm, sglang, etc.).
	EngineType string

	// EngineVersion is the inference engine version.
	EngineVersion string
}

// ParseRuntimeCapabilities extracts capability information from runtime annotations.
func ParseRuntimeCapabilities(annotations map[string]string) *RuntimeCapabilities {
	if annotations == nil {
		return &RuntimeCapabilities{}
	}

	caps := &RuntimeCapabilities{}

	// Parse max context length
	if val, ok := annotations[AnnotationMaxContextLength]; ok {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			caps.MaxContextLength = parsed
		}
	}

	// Parse optimization profiles
	if val, ok := annotations[AnnotationOptimizationProfiles]; ok {
		profiles := strings.Split(val, ",")
		for i, p := range profiles {
			profiles[i] = strings.TrimSpace(p)
		}
		caps.OptimizationProfiles = profiles
	}

	// Parse max throughput
	if val, ok := annotations[AnnotationMaxThroughput]; ok {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			caps.MaxThroughput = parsed
		}
	}

	// Parse typical P99 latency
	if val, ok := annotations[AnnotationTypicalP99Latency]; ok {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			caps.TypicalP99Latency = parsed
		}
	}

	// Parse max concurrency
	if val, ok := annotations[AnnotationMaxConcurrency]; ok {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			caps.MaxConcurrency = parsed
		}
	}

	// Parse cost tier
	if val, ok := annotations[AnnotationCostTier]; ok {
		caps.CostTier = strings.ToLower(strings.TrimSpace(val))
	}

	// Parse engine type
	if val, ok := annotations[AnnotationEngineType]; ok {
		caps.EngineType = strings.ToLower(strings.TrimSpace(val))
	}

	// Parse engine version
	if val, ok := annotations[AnnotationEngineVersion]; ok {
		caps.EngineVersion = strings.TrimSpace(val)
	}

	return caps
}

// SupportsOptimizationProfile checks if the runtime supports a specific optimization profile.
func (c *RuntimeCapabilities) SupportsOptimizationProfile(profile string) bool {
	if len(c.OptimizationProfiles) == 0 {
		// If no profiles specified, assume it supports Balanced
		return profile == "Balanced"
	}
	for _, p := range c.OptimizationProfiles {
		if strings.EqualFold(p, profile) {
			return true
		}
	}
	return false
}

// MeetsContextLengthRequirement checks if the runtime meets the context length requirement.
func (c *RuntimeCapabilities) MeetsContextLengthRequirement(required int64) bool {
	if c.MaxContextLength == 0 {
		// If not specified, assume it can handle any context length
		return true
	}
	return c.MaxContextLength >= required
}

// MeetsThroughputRequirement checks if the runtime meets the throughput requirement.
func (c *RuntimeCapabilities) MeetsThroughputRequirement(required int64) bool {
	if c.MaxThroughput == 0 {
		// If not specified, we can't verify - assume it meets requirement
		return true
	}
	return c.MaxThroughput >= required
}

// MeetsLatencyRequirement checks if the runtime meets the latency requirement.
func (c *RuntimeCapabilities) MeetsLatencyRequirement(maxLatency int64) bool {
	if c.TypicalP99Latency == 0 {
		// If not specified, we can't verify - assume it meets requirement
		return true
	}
	return c.TypicalP99Latency <= maxLatency
}

// MeetsConcurrencyRequirement checks if the runtime meets the concurrency requirement.
func (c *RuntimeCapabilities) MeetsConcurrencyRequirement(required int64) bool {
	if c.MaxConcurrency == 0 {
		// If not specified, we can't verify - assume it meets requirement
		return true
	}
	return c.MaxConcurrency >= required
}

// IsCostOptimized returns true if this runtime is considered cost-optimized.
func (c *RuntimeCapabilities) IsCostOptimized() bool {
	return c.CostTier == CostTierLow
}
