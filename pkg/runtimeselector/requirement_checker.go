package runtimeselector

import (
	"fmt"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// Requirement scoring weights
const (
	// ContextLengthWeight is the score weight for context length requirement match
	ContextLengthWeight = 5.0

	// OptimizationMatchWeight is the score weight for optimization profile match
	OptimizationMatchWeight = 10.0

	// ThroughputWeight is the score weight for throughput requirement match
	ThroughputWeight = 3.0

	// LatencyWeight is the score weight for latency requirement match
	LatencyWeight = 4.0

	// ConcurrencyWeight is the score weight for concurrency requirement match
	ConcurrencyWeight = 2.0
)

// DefaultRequirementChecker implements RequirementChecker with standard requirement validation.
type DefaultRequirementChecker struct {
	config *Config
}

// NewDefaultRequirementChecker creates a new DefaultRequirementChecker.
func NewDefaultRequirementChecker(config *Config) RequirementChecker {
	return &DefaultRequirementChecker{
		config: config,
	}
}

// CheckRequirements validates if a runtime meets the specified requirements.
func (c *DefaultRequirementChecker) CheckRequirements(
	runtime *v1beta1.ServingRuntimeSpec,
	annotations map[string]string,
	requirements *v1beta1.ServiceRequirements,
) (*RequirementMatch, bool) {
	// If no requirements specified, return full match
	if requirements == nil {
		return &RequirementMatch{
			ContextLengthMet:         true,
			OptimizationProfileMatch: true,
			ThroughputMet:            true,
			LatencyMet:               true,
			ConcurrencyMet:           true,
			Score:                    0, // No bonus score when no requirements
			Reasons:                  []string{"No requirements specified"},
		}, true
	}

	// Parse runtime capabilities from annotations
	caps := ParseRuntimeCapabilities(annotations)

	match := &RequirementMatch{
		ContextLengthMet:         true,
		OptimizationProfileMatch: true,
		ThroughputMet:            true,
		LatencyMet:               true,
		ConcurrencyMet:           true,
		Reasons:                  []string{},
	}

	allMet := true

	// Check context length requirement
	if requirements.MaxContextLength != nil && *requirements.MaxContextLength > 0 {
		if caps.MeetsContextLengthRequirement(*requirements.MaxContextLength) {
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("Context length requirement met: %d <= %d", *requirements.MaxContextLength, caps.MaxContextLength))
		} else {
			match.ContextLengthMet = false
			allMet = false
			if caps.MaxContextLength > 0 {
				match.Reasons = append(match.Reasons,
					fmt.Sprintf("Context length requirement NOT met: required %d > available %d",
						*requirements.MaxContextLength, caps.MaxContextLength))
			} else {
				match.Reasons = append(match.Reasons,
					fmt.Sprintf("Context length requirement NOT met: required %d, runtime does not specify max context",
						*requirements.MaxContextLength))
			}
		}
	}

	// Check optimization profile match
	if requirements.OptimizationPolicy != "" {
		profileStr := string(requirements.OptimizationPolicy)
		if caps.SupportsOptimizationProfile(profileStr) {
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("Optimization profile '%s' supported", profileStr))
		} else {
			match.OptimizationProfileMatch = false
			// Optimization profile mismatch is a soft requirement - don't fail hard
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("Optimization profile '%s' not explicitly supported (supported: %v)",
					profileStr, caps.OptimizationProfiles))
		}
	}

	// Check throughput requirement
	if requirements.MinThroughput != nil && *requirements.MinThroughput > 0 {
		if caps.MeetsThroughputRequirement(*requirements.MinThroughput) {
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("Throughput requirement met: %d >= %d tokens/s", caps.MaxThroughput, *requirements.MinThroughput))
		} else {
			match.ThroughputMet = false
			// Throughput is a soft requirement
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("Throughput requirement NOT met: required %d > available %d tokens/s",
					*requirements.MinThroughput, caps.MaxThroughput))
		}
	}

	// Check latency requirement
	if requirements.MaxP99Latency != nil && *requirements.MaxP99Latency > 0 {
		if caps.MeetsLatencyRequirement(*requirements.MaxP99Latency) {
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("Latency requirement met: %d <= %d ms", caps.TypicalP99Latency, *requirements.MaxP99Latency))
		} else {
			match.LatencyMet = false
			// Latency is a soft requirement
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("Latency requirement NOT met: typical %d > required %d ms",
					caps.TypicalP99Latency, *requirements.MaxP99Latency))
		}
	}

	// Check concurrency requirement
	if requirements.MaxConcurrency != nil && *requirements.MaxConcurrency > 0 {
		if caps.MeetsConcurrencyRequirement(*requirements.MaxConcurrency) {
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("Concurrency requirement met: %d >= %d", caps.MaxConcurrency, *requirements.MaxConcurrency))
		} else {
			match.ConcurrencyMet = false
			// Concurrency is a soft requirement
			match.Reasons = append(match.Reasons,
				fmt.Sprintf("Concurrency requirement NOT met: required %d > available %d",
					*requirements.MaxConcurrency, caps.MaxConcurrency))
		}
	}

	// Calculate the requirement score
	match.Score = c.CalculateRequirementScore(match, requirements)

	return match, allMet
}

// CalculateRequirementScore calculates a score based on how well requirements are met.
func (c *DefaultRequirementChecker) CalculateRequirementScore(
	match *RequirementMatch,
	requirements *v1beta1.ServiceRequirements,
) float64 {
	if requirements == nil || match == nil {
		return 0
	}

	var score float64

	// Context length score
	if requirements.MaxContextLength != nil && *requirements.MaxContextLength > 0 {
		if match.ContextLengthMet {
			score += ContextLengthWeight
		}
	}

	// Optimization profile score (highest weight)
	if requirements.OptimizationPolicy != "" {
		if match.OptimizationProfileMatch {
			score += OptimizationMatchWeight
		}
	}

	// Throughput score
	if requirements.MinThroughput != nil && *requirements.MinThroughput > 0 {
		if match.ThroughputMet {
			score += ThroughputWeight
		}
	}

	// Latency score
	if requirements.MaxP99Latency != nil && *requirements.MaxP99Latency > 0 {
		if match.LatencyMet {
			score += LatencyWeight
		}
	}

	// Concurrency score
	if requirements.MaxConcurrency != nil && *requirements.MaxConcurrency > 0 {
		if match.ConcurrencyMet {
			score += ConcurrencyWeight
		}
	}

	return score
}

// HasHardRequirementFailure checks if any hard requirements failed.
// Currently, only context length is a hard requirement (runtime cannot serve if context is too long).
func HasHardRequirementFailure(match *RequirementMatch) bool {
	if match == nil {
		return false
	}
	// Context length is the only hard requirement - if the runtime can't support the context,
	// it physically cannot serve the request
	return !match.ContextLengthMet
}
