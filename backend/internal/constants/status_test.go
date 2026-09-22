package constants

import "testing"

func TestArtifactTransitionGraph(t *testing.T) {
	if !CanTransition(ArtifactTransitions, "registered", "stable") {
		t.Fatalf("expected registered -> stable transition to be allowed")
	}
	if CanTransition(ArtifactTransitions, "registered", "unknown") {
		t.Fatal("unknown status must never be accepted")
	}
}

func TestStageApprovalCannotSkipReview(t *testing.T) {
	if CanTransition(StageApprovalTransitions, "draft", "approved") {
		t.Fatal("draft approval must not skip independent review")
	}
	if !CanTransition(StageApprovalTransitions, "draft", "review") {
		t.Fatal("draft approval must enter review")
	}
}

func TestStageApprovalCorrectionRoundTrip(t *testing.T) {
	if !CanTransition(StageApprovalTransitions, "review", "pending_correction") {
		t.Fatal("reviewer must be able to return an approval for correction")
	}
	if CanTransition(StageApprovalTransitions, "pending_correction", "approved") {
		t.Fatal("a pending correction must not skip the new review batch")
	}
	if !CanTransition(StageApprovalTransitions, "pending_correction", "review") {
		t.Fatal("operator correction must reopen review")
	}
	if CanTransition(StageApprovalTransitions, "draft", "pending_correction") {
		t.Fatal("only a record under review can be returned for correction")
	}
}
