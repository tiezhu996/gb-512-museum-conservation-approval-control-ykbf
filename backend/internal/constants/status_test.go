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

func TestStageApprovalCorrectionFlow(t *testing.T) {
	if !CanTransition(StageApprovalTransitions, "review", "correction") {
		t.Fatal("reviewer must be able to request correction from review")
	}
	if !CanTransition(StageApprovalTransitions, "correction", "review") {
		t.Fatal("operator correction explanation must reopen review")
	}
	if CanTransition(StageApprovalTransitions, "correction", "approved") {
		t.Fatal("correction must not skip the new review batch")
	}
	if CanTransition(StageApprovalTransitions, "correction", "correction") {
		t.Fatal("an approval awaiting correction cannot request correction again")
	}
	if CanTransition(StageApprovalTransitions, "approved", "correction") {
		t.Fatal("an approved approval is final")
	}
}
