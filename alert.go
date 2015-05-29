package main

import (
	"fmt"
	"hash/fnv"
	"sort"
)

const AlertNameLabel = "alertname"

type AlertFingerprint uint64

type AlertLabelSet map[string]string
type AlertLabelSets []AlertLabelSet

type AlertPayload map[string]string

type Alerts []*Alert

// Alert models an action triggered by Prometheus.
type Alert struct {
	// Short summary of alert.
	Summary string `json:"summary"`
	// Long description of alert.
	Description string `json:"description"`
	// Label value pairs for purpose of aggregation, matching, and disposition
	// dispatching. This must minimally include an "alertname" label.
	Labels AlertLabelSet `json:"labels"`
	// Extra key/value information which is not used for aggregation.
	Payload AlertPayload `json:"payload"`
}

func (a *Alert) Name() string {
	return a.Labels[AlertNameLabel]
}

func (a *Alert) Fingerprint() AlertFingerprint {
	return a.Labels.Fingerprint()
}

func (l AlertLabelSet) Fingerprint() AlertFingerprint {
	keys := []string{}

	for k := range l {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	summer := fnv.New64a()

	separator := string([]byte{0})
	for _, k := range keys {
		fmt.Fprintf(summer, "%s%s%s%s", k, separator, l[k], separator)
	}

	return AlertFingerprint(summer.Sum64())
}

func (l AlertLabelSet) Equal(o AlertLabelSet) bool {
	if len(l) != len(o) {
		return false
	}
	for k, v := range l {
		if o[k] != v {
			return false
		}
	}
	return true
}

func (l AlertLabelSet) MatchOnLabels(o AlertLabelSet, labels []string) bool {
	for _, k := range labels {
		if l[k] != o[k] {
			return false
		}
	}
	return true
}
