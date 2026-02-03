# Enhanced Runtime Selector: Workload-Aware Runtime Selection

## Overview

This document describes the enhanced runtime selection system for OME (Open Model Engine) that enables intelligent, workload-specific runtime selection based on user requirements, performance profiles, and optimization policies.

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Current Implementation](#current-implementation)
3. [Proposed Enhancements](#proposed-enhancements)
4. [Architecture](#architecture)
5. [Implementation Details](#implementation-details)
6. [Usage Examples](#usage-examples)
7. [Migration Guide](#migration-guide)

---

## Problem Statement

### Current Limitations

The existing runtime selection system in OME selects a ServingRuntime based on:
- Model format compatibility (e.g., vLLM, ONNX, TensorRT)
- Model framework compatibility (e.g., PyTorch, TensorFlow)
- Accelerator class matching (e.g., GPU type)
- Model size range constraints

**However, it lacks:**
1. **Workload-aware selection** - No consideration for latency vs throughput requirements
2. **Performance SLA matching** - Cannot express "I need P99 latency < 100ms"
3. **Cost optimization** - No preference for cost-efficient runtimes
4. **Dynamic parameter injection** - Engine arguments are static, not adapted to workload

### Real-World Scenarios Not Supported

| Scenario | Current Behavior | Desired Behavior |
|----------|------------------|------------------|
| Real-time chatbot | Picks any compatible runtime | Pick runtime optimized for low latency |
| Batch processing | Same as above | Pick runtime optimized for throughput |
| Cost-sensitive workload | No consideration | Prefer runtimes that minimize GPU usage |
| High-context RAG | May pick runtime with low context limit | Pick runtime supporting required context length |

---

## Current Implementation

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     Runtime Selector Flow                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌───────────┐ │
│  │ Fetcher  │───▶│ Matcher  │───▶│  Scorer  │───▶│ Selector  │ │
│  └──────────┘    └──────────┘    └──────────┘    └───────────┘ │
│       │               │               │               │         │
│       ▼               ▼               ▼               ▼         │
│  Fetch all       Check basic    Calculate      Select best      │
│  runtimes        compatibility  weighted       runtime          │
│                                 scores                          │
└─────────────────────────────────────────────────────────────────┘
```

### Component Details

#### 1. RuntimeFetcher (`pkg/runtimeselector/fetcher.go`)

Fetches all available runtimes from Kubernetes:
- **ClusterServingRuntime** (cluster-scoped)
- **ServingRuntime** (namespace-scoped)

```go
type RuntimeFetcher interface {
    FetchRuntimes(ctx context.Context, namespace string) ([]servingv1beta1.ServingRuntimeSpec, error)
}
```

#### 2. RuntimeMatcher (`pkg/runtimeselector/matcher.go`)

Checks if a runtime is compatible with the model:

| Check | Description | Priority |
|-------|-------------|----------|
| Disabled | Skip if `spec.disabled: true` | Hard filter |
| Accelerator Class | Match GPU type requirements | Hard filter |
| Model Format | vLLM, ONNX, TensorRT, etc. | Scored match |
| Model Framework | PyTorch, TensorFlow, etc. | Scored match |
| Architecture | x86_64, arm64, etc. | Hard filter |
| Quantization | GPTQ, AWQ, INT8, etc. | Soft match |
| Model Size | Within min/max range | Hard filter |

```go
type RuntimeMatcher interface {
    CheckCompatibility(runtime servingv1beta1.ServingRuntimeSpec, 
                      model *servingv1beta1.BaseModelSpec,
                      config Config) (MatchDetails, bool)
}
```

#### 3. RuntimeScorer (`pkg/runtimeselector/scorer.go`)

Calculates compatibility scores using weighted formula:

```
Score = Σ(formatWeight × priority) + Σ(frameworkWeight × priority)
```

**Default Weights:**
- `ModelFormatWeight = 10`
- `ModelFrameworkWeight = 5`
- `DefaultPriority = 1`

**Tie-breaking (in order):**
1. **Size Fit Score** - Tighter size range wins
2. **Namespace Preference** - Namespace-scoped over cluster-scoped
3. **Alphabetical Name** - Deterministic ordering

```go
// Size fit score calculation
sizeFitScore = 1.0 / (1.0 + (maxSize - minSize) / modelSize)
```

#### 4. RuntimeSelector (`pkg/runtimeselector/selector.go`)

Orchestrates the selection process:

```go
func (s *defaultSelector) SelectRuntime(ctx context.Context, 
    model *servingv1beta1.BaseModelSpec,
    namespace string, 
    runtimeName string) (*RuntimeSelection, error)
```

### Current Selection Algorithm

```
1. Fetch all runtimes (cluster + namespace)
2. For each runtime:
   a. Skip if disabled
   b. Check accelerator compatibility
   c. Check format/framework match
   d. Check size constraints
   e. Calculate weighted score
3. Sort by: score DESC, sizeFit DESC, namespace preference, name ASC
4. Return top result
```

---

## Proposed Enhancements

### Feature 1: Workload-Aware Runtime Selection

Allow users to specify workload requirements in InferenceService:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: my-chatbot
spec:
  model: meta-llama/Llama-3.1-8B-Instruct
  requirements:
    optimizationPolicy: LatencyOptimized  # or ThroughputOptimized, CostOptimized, Balanced
    maxContextLength: 32768
    minThroughput: 100    # tokens/second
    maxP99Latency: 200    # milliseconds
    maxConcurrency: 50
```

### Feature 2: Runtime Capability Annotations

Runtimes declare their capabilities via annotations:

```yaml
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: vllm-latency-optimized
  annotations:
    ome.io/max-context-length: "131072"
    ome.io/optimization-profiles: "LatencyOptimized,Balanced"
    ome.io/max-throughput: "500"
    ome.io/typical-p99-latency: "50"
    ome.io/max-concurrency: "100"
spec:
  supportedModelFormats:
    - name: vLLM
      priority: 2
  # ...
```

### Feature 3: Dynamic Parameter Injection

Engine arguments are automatically injected based on:
1. Selected optimization profile
2. User requirements
3. Runtime capabilities

```go
// Before injection
container.Args = ["--model", "/models/llama"]

// After injection (LatencyOptimized profile)
container.Args = [
    "--model", "/models/llama",
    "--enable-chunked-prefill",
    "--max-num-batched-tokens", "2048",
    "--max-model-len", "32768",
    "--max-num-seqs", "50",
]
```

---

## Architecture

### Enhanced Selection Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Enhanced Runtime Selector Flow                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────┐   ┌──────────┐   ┌───────────┐   ┌──────────┐   ┌──────────┐ │
│  │ Fetcher  │──▶│ Matcher  │──▶│Requirement│──▶│  Scorer  │──▶│ Selector │ │
│  └──────────┘   └──────────┘   │  Checker  │   └──────────┘   └──────────┘ │
│       │              │         └───────────┘        │              │        │
│       ▼              ▼              │               ▼              ▼        │
│  Fetch all      Basic checks   NEW: Check      Enhanced       Select +      │
│  runtimes       + annotations  requirements    scoring        inject args   │
│                                                                              │
│                                      │                              │        │
│                                      ▼                              ▼        │
│                              ┌───────────────┐            ┌──────────────┐  │
│                              │  Requirement  │            │  Parameter   │  │
│                              │   Matching    │            │  Injector    │  │
│                              └───────────────┘            └──────────────┘  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### New Components

#### 1. RequirementChecker

Validates if runtime meets user requirements:

```go
type RequirementChecker interface {
    CheckRequirements(runtime servingv1beta1.ServingRuntimeSpec,
                     requirements *ServiceRequirements) (RequirementMatch, bool)
}

type RequirementMatch struct {
    ContextLengthMet   bool
    OptimizationMatch  bool
    ThroughputMet      bool
    LatencyMet         bool
    ConcurrencyMet     bool
    Score              float64
}
```

#### 2. ParameterInjector

Injects optimization arguments based on profile:

```go
type ParameterInjector interface {
    InjectParameters(runtime *servingv1beta1.ServingRuntimeSpec,
                    requirements *ServiceRequirements) error
}
```

---

## Implementation Details

### Data Types

#### ServiceRequirements

```go
// pkg/apis/ome/v1beta1/inference_service.go

type ServiceRequirements struct {
    // OptimizationPolicy specifies the optimization strategy
    // +kubebuilder:validation:Enum=LatencyOptimized;ThroughputOptimized;CostOptimized;Balanced
    OptimizationPolicy OptimizationPolicy `json:"optimizationPolicy,omitempty"`

    // MaxContextLength specifies the maximum context length required
    MaxContextLength *int64 `json:"maxContextLength,omitempty"`

    // MinThroughput specifies minimum tokens/second required
    MinThroughput *int64 `json:"minThroughput,omitempty"`

    // MaxP99Latency specifies maximum P99 latency in milliseconds
    MaxP99Latency *int64 `json:"maxP99Latency,omitempty"`

    // MaxConcurrency specifies maximum concurrent requests
    MaxConcurrency *int64 `json:"maxConcurrency,omitempty"`
}

type OptimizationPolicy string

const (
    LatencyOptimized    OptimizationPolicy = "LatencyOptimized"
    ThroughputOptimized OptimizationPolicy = "ThroughputOptimized"
    CostOptimized       OptimizationPolicy = "CostOptimized"
    Balanced            OptimizationPolicy = "Balanced"
)
```

#### Runtime Annotations

| Annotation | Type | Description |
|------------|------|-------------|
| `ome.io/max-context-length` | int | Maximum supported context length |
| `ome.io/optimization-profiles` | string | Comma-separated list of supported profiles |
| `ome.io/max-throughput` | int | Maximum throughput in tokens/second |
| `ome.io/typical-p99-latency` | int | Typical P99 latency in milliseconds |
| `ome.io/max-concurrency` | int | Maximum concurrent requests |

### Enhanced Scoring Formula

```
TotalScore = BaseScore + RequirementScore

Where:
  BaseScore = Σ(formatWeight × priority) + Σ(frameworkWeight × priority)
  
  RequirementScore = (contextLengthScore × 5) + 
                     (optimizationMatchScore × 10) +
                     (throughputScore × 3) +
                     (latencyScore × 4) +
                     (concurrencyScore × 2)
```

### Optimization Profiles

#### vLLM Profiles

| Profile | Arguments | Use Case |
|---------|-----------|----------|
| LatencyOptimized | `--enable-chunked-prefill`, `--max-num-batched-tokens=2048` | Real-time chat, interactive applications |
| ThroughputOptimized | `--disable-log-requests`, `--max-num-seqs=256`, `--gpu-memory-utilization=0.95` | Batch processing, offline inference |
| CostOptimized | `--max-num-seqs=128`, `--gpu-memory-utilization=0.85` | Budget-constrained workloads |
| Balanced | Default settings | General-purpose workloads |

#### SGLang Profiles

| Profile | Arguments | Use Case |
|---------|-----------|----------|
| LatencyOptimized | `--chunked-prefill-size=2048`, `--schedule-policy=fcfs` | Real-time applications |
| ThroughputOptimized | `--schedule-policy=lpm`, `--max-running-requests=256` | High-volume batch processing |
| CostOptimized | `--max-running-requests=128`, `--mem-fraction-static=0.85` | Cost-sensitive deployments |
| Balanced | Default settings | General-purpose workloads |

---

## Usage Examples

### Scenario 1: Real-Time Chatbot (Latency-Critical)

**User Requirements:**
- Minimum latency for responsive chat experience
- Context length of 8K tokens
- Handle 50 concurrent users

**InferenceService:**
```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: chatbot-llama
  namespace: production
spec:
  model: meta-llama/Llama-3.1-8B-Instruct
  requirements:
    optimizationPolicy: LatencyOptimized
    maxContextLength: 8192
    maxP99Latency: 100        # 100ms target
    maxConcurrency: 50
```

**Available Runtimes:**

```yaml
# Runtime A: vllm-standard
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: vllm-standard
  annotations:
    ome.io/max-context-length: "32768"
    ome.io/optimization-profiles: "Balanced"
    ome.io/typical-p99-latency: "150"
spec:
  supportedModelFormats:
    - name: vLLM
      priority: 1

---
# Runtime B: vllm-latency-optimized
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: vllm-latency-optimized
  annotations:
    ome.io/max-context-length: "32768"
    ome.io/optimization-profiles: "LatencyOptimized,Balanced"
    ome.io/typical-p99-latency: "50"
    ome.io/max-concurrency: "100"
spec:
  supportedModelFormats:
    - name: vLLM
      priority: 2
```

**Selection Process:**

| Runtime | Base Score | Requirement Score | Total | Selected |
|---------|------------|-------------------|-------|----------|
| vllm-standard | 10 | 0 (latency exceeded) | 10 | ❌ |
| vllm-latency-optimized | 20 | 24 (full match) | 44 | ✅ |

**Result:** `vllm-latency-optimized` selected with injected arguments:
```
--enable-chunked-prefill
--max-num-batched-tokens=2048
--max-model-len=8192
--max-num-seqs=50
```

---

### Scenario 2: Batch Document Processing (Throughput-Critical)

**User Requirements:**
- Maximum throughput for processing large document batches
- Long context for full documents (32K)
- Cost is secondary concern

**InferenceService:**
```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: doc-processor
  namespace: batch-jobs
spec:
  model: meta-llama/Llama-3.1-70B-Instruct
  requirements:
    optimizationPolicy: ThroughputOptimized
    maxContextLength: 32768
    minThroughput: 200
```

**Available Runtimes:**

```yaml
# Runtime A: vllm-throughput
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: vllm-throughput
  annotations:
    ome.io/max-context-length: "65536"
    ome.io/optimization-profiles: "ThroughputOptimized"
    ome.io/max-throughput: "500"
spec:
  supportedModelFormats:
    - name: vLLM
      priority: 2

---
# Runtime B: sglang-throughput
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: sglang-throughput
  annotations:
    ome.io/max-context-length: "131072"
    ome.io/optimization-profiles: "ThroughputOptimized,LatencyOptimized"
    ome.io/max-throughput: "800"
spec:
  supportedModelFormats:
    - name: srt
      priority: 3
```

**Selection Process:**

| Runtime | Base Score | Requirement Score | Total | Selected |
|---------|------------|-------------------|-------|----------|
| vllm-throughput | 20 | 18 | 38 | ❌ |
| sglang-throughput | 30 | 23 (higher throughput) | 53 | ✅ |

**Result:** `sglang-throughput` selected with injected arguments:
```
--schedule-policy=lpm
--max-running-requests=256
--context-length=32768
```

---

### Scenario 3: Budget-Constrained Startup (Cost-Critical)

**User Requirements:**
- Minimize GPU costs
- Acceptable to trade off some performance
- Modest context requirements

**InferenceService:**
```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: budget-assistant
  namespace: startup-app
spec:
  model: mistralai/Mistral-7B-Instruct-v0.3
  requirements:
    optimizationPolicy: CostOptimized
    maxContextLength: 4096
    maxConcurrency: 20
```

**Available Runtimes:**

```yaml
# Runtime A: vllm-standard (A100)
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: vllm-a100
  annotations:
    ome.io/optimization-profiles: "Balanced,ThroughputOptimized"
    ome.io/cost-tier: "high"
spec:
  acceleratorRequirements:
    - type: nvidia-a100

---
# Runtime B: vllm-cost-optimized (T4)
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: vllm-t4-cost
  annotations:
    ome.io/optimization-profiles: "CostOptimized,Balanced"
    ome.io/cost-tier: "low"
    ome.io/max-context-length: "8192"
spec:
  acceleratorRequirements:
    - type: nvidia-t4
```

**Selection Process:**

| Runtime | Base Score | Requirement Score | Total | Selected |
|---------|------------|-------------------|-------|----------|
| vllm-a100 | 10 | 0 (no CostOptimized) | 10 | ❌ |
| vllm-t4-cost | 10 | 15 (CostOptimized match) | 25 | ✅ |

**Result:** `vllm-t4-cost` selected with injected arguments:
```
--max-num-seqs=20
--gpu-memory-utilization=0.85
--max-model-len=4096
```

---

### Scenario 4: RAG Application (Context-Critical)

**User Requirements:**
- Very long context for document retrieval + generation
- Balanced performance
- Support for 100K+ context

**InferenceService:**
```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: rag-engine
  namespace: enterprise
spec:
  model: meta-llama/Llama-3.1-8B-Instruct
  requirements:
    optimizationPolicy: Balanced
    maxContextLength: 100000
```

**Available Runtimes:**

```yaml
# Runtime A: vllm-standard (32K max)
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: vllm-32k
  annotations:
    ome.io/max-context-length: "32768"
    ome.io/optimization-profiles: "Balanced,LatencyOptimized"

---
# Runtime B: sglang-long-context (128K max)
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: sglang-128k
  annotations:
    ome.io/max-context-length: "131072"
    ome.io/optimization-profiles: "Balanced,ThroughputOptimized"
```

**Selection Process:**

| Runtime | Context Check | Requirement Score | Selected |
|---------|---------------|-------------------|----------|
| vllm-32k | ❌ (32K < 100K) | DISQUALIFIED | ❌ |
| sglang-128k | ✅ (128K ≥ 100K) | 20 | ✅ |

**Result:** `sglang-128k` selected (only runtime meeting context requirement)

---

### Scenario 5: Mixed Workload (Default/No Requirements)

**User Requirements:**
- No specific requirements specified
- Let system choose best default

**InferenceService:**
```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: general-llm
spec:
  model: meta-llama/Llama-3.1-8B-Instruct
  # No requirements specified
```

**Behavior:** Falls back to existing selection algorithm:
1. Filter by format/framework compatibility
2. Score by priority weights
3. Tie-break by size fit, namespace, name

This ensures backward compatibility with existing deployments.

---

### Scenario 6: Explicit Runtime Override

**User Requirements:**
- User knows exactly which runtime they want
- Skip automatic selection

**InferenceService:**
```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: explicit-runtime
spec:
  model: meta-llama/Llama-3.1-8B-Instruct
  runtime: vllm-latency-optimized  # Explicit runtime
  requirements:
    optimizationPolicy: LatencyOptimized
    maxContextLength: 8192
```

**Behavior:**
1. Skip runtime selection
2. Validate specified runtime exists and is compatible
3. Still inject parameters based on requirements
4. Warn if runtime doesn't support the optimization profile

---

## Migration Guide

### For Existing Deployments

**No action required.** Existing InferenceService resources without `requirements` field will continue to work with the existing selection algorithm.

### For New Deployments

1. **Add requirements to InferenceService** (optional):
```yaml
spec:
  requirements:
    optimizationPolicy: Balanced
```

2. **Annotate existing runtimes** (recommended):
```yaml
metadata:
  annotations:
    ome.io/max-context-length: "32768"
    ome.io/optimization-profiles: "Balanced,LatencyOptimized"
```

### Runtime Operator Checklist

- [ ] Review existing ClusterServingRuntime resources
- [ ] Add capability annotations based on actual performance characteristics
- [ ] Test selection with sample InferenceService resources
- [ ] Monitor selection decisions in controller logs

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RUNTIME_SELECTOR_LOG_LEVEL` | `info` | Logging verbosity for selection decisions |
| `REQUIREMENT_STRICT_MODE` | `false` | If true, fail if no runtime meets all requirements |

### Controller Flags

```bash
--runtime-selector-weights="format=10,framework=5,context=5,optimization=10"
--enable-parameter-injection=true
--default-optimization-policy=Balanced
```

---

## Observability

### Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `ome_runtime_selection_total` | Counter | Total runtime selections by result |
| `ome_runtime_selection_duration_seconds` | Histogram | Time to select runtime |
| `ome_runtime_requirement_match` | Gauge | Requirements met by selected runtime |
| `ome_parameter_injection_total` | Counter | Parameter injections by profile |

### Events

```
Normal  RuntimeSelected    Selected runtime 'vllm-latency-optimized' for InferenceService 'chatbot'
Normal  ParametersInjected Injected 4 optimization parameters for profile 'LatencyOptimized'
Warning RequirementPartial Runtime 'vllm-standard' meets 3/5 requirements
```

---

## Future Enhancements

### Phase 2: Dynamic Runtime Switching

- Monitor actual performance metrics
- Automatically switch runtime if SLAs not met
- Gradual traffic migration during switch

### Phase 3: Cost-Aware Scheduling

- Integration with cloud cost APIs
- Spot instance preference for cost-optimized workloads
- Multi-cluster runtime selection

### Phase 4: Custom Scoring Plugins

- User-defined scoring functions
- Webhook-based requirement validation
- ML-based runtime recommendation

---

## References

- [vLLM Engine Arguments](https://docs.vllm.ai/en/latest/serving/engine_args.html)
- [SGLang Server Arguments](https://sgl-project.github.io/references/server_args.html)
- [Kubernetes Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/)
- [KServe ServingRuntime](https://kserve.github.io/website/latest/modelserving/servingruntimes/)

---

## Appendix A: Complete Annotation Reference

```yaml
metadata:
  annotations:
    # Performance Capabilities
    ome.io/max-context-length: "131072"
    ome.io/optimization-profiles: "LatencyOptimized,ThroughputOptimized,CostOptimized,Balanced"
    ome.io/max-throughput: "500"          # tokens/second
    ome.io/typical-p99-latency: "50"      # milliseconds
    ome.io/max-concurrency: "100"         # concurrent requests
    
    # Cost Information
    ome.io/cost-tier: "low|medium|high"
    ome.io/gpu-memory-required: "40Gi"
    
    # Engine Information
    ome.io/engine-type: "vllm|sglang|triton|tgi"
    ome.io/engine-version: "0.6.0"
```

## Appendix B: Optimization Profile Arguments

### vLLM Complete Profile Arguments

```go
var VLLMOptimizationProfiles = map[OptimizationPolicy][]string{
    LatencyOptimized: {
        "--enable-chunked-prefill",
        "--max-num-batched-tokens=2048",
        "--scheduling-policy=fcfs",
    },
    ThroughputOptimized: {
        "--disable-log-requests",
        "--max-num-seqs=256",
        "--gpu-memory-utilization=0.95",
        "--enable-prefix-caching",
    },
    CostOptimized: {
        "--max-num-seqs=128",
        "--gpu-memory-utilization=0.85",
    },
    Balanced: {}, // Use vLLM defaults
}
```

### SGLang Complete Profile Arguments

```go
var SGLangOptimizationProfiles = map[OptimizationPolicy][]string{
    LatencyOptimized: {
        "--chunked-prefill-size=2048",
        "--schedule-policy=fcfs",
    },
    ThroughputOptimized: {
        "--schedule-policy=lpm",
        "--max-running-requests=256",
        "--mem-fraction-static=0.95",
    },
    CostOptimized: {
        "--max-running-requests=128",
        "--mem-fraction-static=0.85",
    },
    Balanced: {}, // Use SGLang defaults
}
```
