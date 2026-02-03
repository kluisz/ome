# Enhanced Runtime Selection - Sample Deployment

This directory contains sample YAML files to test the enhanced runtime selection feature.

## Overview

The enhanced runtime selection allows InferenceServices to specify workload requirements, and the system automatically selects the best matching runtime based on:
- Optimization policy (LatencyOptimized, ThroughputOptimized, CostOptimized, Balanced)
- Context length requirements
- Throughput requirements
- Latency requirements
- Concurrency requirements

## Files

| File | Description |
|------|-------------|
| `base-model.yaml` | ClusterBaseModel for Llama-3.2-3B |
| `runtime-latency-optimized.yaml` | Runtime optimized for low latency (chatbots) |
| `runtime-throughput-optimized.yaml` | Runtime optimized for high throughput (batch) |
| `runtime-cost-optimized.yaml` | Runtime optimized for cost efficiency |
| `isvc-latency-optimized.yaml` | InferenceService requesting latency optimization |
| `isvc-throughput-optimized.yaml` | InferenceService requesting throughput optimization |
| `isvc-cost-optimized.yaml` | InferenceService requesting cost optimization |

## Deployment Steps

### Step 1: Deploy the base resources (model + runtimes)

```bash
kubectl apply -k config/samples/enhanced-runtime-selection/
```

Or individually:
```bash
kubectl apply -f config/samples/enhanced-runtime-selection/base-model.yaml
kubectl apply -f config/samples/enhanced-runtime-selection/runtime-latency-optimized.yaml
kubectl apply -f config/samples/enhanced-runtime-selection/runtime-throughput-optimized.yaml
kubectl apply -f config/samples/enhanced-runtime-selection/runtime-cost-optimized.yaml
```

### Step 2: Verify runtimes are created

```bash
kubectl get clusterservingruntimes
```

Expected output:
```
NAME                        AGE
vllm-latency-optimized      10s
vllm-throughput-optimized   10s
vllm-cost-optimized         10s
```

### Step 3: Deploy an InferenceService with requirements

Choose one based on your use case:

**For a real-time chatbot (low latency):**
```bash
kubectl apply -f config/samples/enhanced-runtime-selection/isvc-latency-optimized.yaml
```
Expected: Selects `vllm-latency-optimized` runtime

**For batch processing (high throughput):**
```bash
kubectl apply -f config/samples/enhanced-runtime-selection/isvc-throughput-optimized.yaml
```
Expected: Selects `vllm-throughput-optimized` runtime

**For budget-constrained workload (cost optimized):**
```bash
kubectl apply -f config/samples/enhanced-runtime-selection/isvc-cost-optimized.yaml
```
Expected: Selects `vllm-cost-optimized` runtime

### Step 4: Verify runtime selection

```bash
kubectl get inferenceservice -o yaml | grep -A5 "status:"
```

Or check controller logs:
```bash
kubectl logs -n ome-system deployment/ome-controller-manager | grep -i "runtime"
```

## Runtime Annotation Reference

| Annotation | Description | Example |
|------------|-------------|---------|
| `ome.io/max-context-length` | Maximum context length in tokens | `"32768"` |
| `ome.io/optimization-profiles` | Comma-separated optimization profiles | `"LatencyOptimized,Balanced"` |
| `ome.io/max-throughput` | Max throughput in tokens/second | `"500"` |
| `ome.io/typical-p99-latency` | Typical P99 latency in milliseconds | `"50"` |
| `ome.io/max-concurrency` | Max concurrent requests | `"100"` |
| `ome.io/cost-tier` | Cost classification | `"low"`, `"medium"`, `"high"` |
| `ome.io/engine-type` | Inference engine type | `"vllm"`, `"sglang"`, `"tgi"` |

## Cleanup

```bash
kubectl delete inferenceservice llama-chatbot-latency llama-batch-processor llama-budget-assistant
kubectl delete -k config/samples/enhanced-runtime-selection/
```
