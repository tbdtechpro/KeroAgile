package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tbdtechpro/KeroAgile/internal/domain"
	"github.com/tbdtechpro/KeroAgile/internal/git"
)

func intPtr(n int) *int { return &n }

func statusPtr(s domain.Status) *domain.Status { return &s }

func TestDecideTransition(t *testing.T) {
	open7 := &git.PRStatus{State: "OPEN", Number: 7}
	merged7 := &git.PRStatus{State: "MERGED", Number: 7}
	closed7 := &git.PRStatus{State: "CLOSED", Number: 7}

	cases := []struct {
		name          string
		currentStatus domain.Status
		currentPR     *int
		pr            *git.PRStatus
		wantStatus    *domain.Status
		wantLinkPR    *int
		wantMerged    bool
	}{
		{
			name: "backlog + open PR -> review, link PR",
			currentStatus: domain.StatusBacklog, currentPR: nil, pr: open7,
			wantStatus: statusPtr(domain.StatusReview), wantLinkPR: intPtr(7),
		},
		{
			name: "todo + open PR -> review, link PR",
			currentStatus: domain.StatusTodo, currentPR: nil, pr: open7,
			wantStatus: statusPtr(domain.StatusReview), wantLinkPR: intPtr(7),
		},
		{
			name: "in_progress + open PR already linked -> review, no re-link",
			currentStatus: domain.StatusInProgress, currentPR: intPtr(7), pr: open7,
			wantStatus: statusPtr(domain.StatusReview), wantLinkPR: nil,
		},
		{
			name: "review + open PR -> no change",
			currentStatus: domain.StatusReview, currentPR: intPtr(7), pr: open7,
			wantStatus: nil, wantLinkPR: nil,
		},
		{
			name: "backlog + merged PR -> link and mark merged",
			currentStatus: domain.StatusBacklog, currentPR: nil, pr: merged7,
			wantLinkPR: intPtr(7), wantMerged: true,
		},
		{
			name: "review + merged PR -> mark merged",
			currentStatus: domain.StatusReview, currentPR: intPtr(7), pr: merged7,
			wantMerged: true,
		},
		{
			name: "done + merged PR -> no change",
			currentStatus: domain.StatusDone, currentPR: intPtr(7), pr: merged7,
		},
		{
			name: "in_progress + no PR -> no change",
			currentStatus: domain.StatusInProgress, currentPR: nil, pr: nil,
		},
		{
			name: "in_progress + closed PR -> no change",
			currentStatus: domain.StatusInProgress, currentPR: intPtr(7), pr: closed7,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decideTransition(tc.currentStatus, tc.currentPR, tc.pr)
			assert.Equal(t, tc.wantStatus, got.newStatus)
			assert.Equal(t, tc.wantLinkPR, got.linkPR)
			assert.Equal(t, tc.wantMerged, got.markMerged)
		})
	}
}
