package trader

import "testing"

func TestMaxSingleBetByDeployable(t *testing.T) {
	itemMin := 2.0

	tests := []struct {
		name       string
		deployable float64
		itemMin    float64
		want       float64
	}{
		{"zero deployable", 0, itemMin, 0},
		{"below item min", 5, 6, 0},
		{"exactly 1x floor", 2, itemMin, 2},
		{"cap at 3x min", 50, itemMin, 6},
		{"large deployable still 3x", 1200, itemMin, 6},
		{"deployable between 1x and 3x", 5, itemMin, 5},
		// 12% of $40 = $4.8 < 3×($6) → fraction binds
		{"cash fraction binds", 40, itemMin, 4.8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxSingleBetByDeployable(tt.deployable, tt.itemMin)
			if got != tt.want {
				t.Fatalf("MaxSingleBetByDeployable(%v, %v) = %v, want %v", tt.deployable, tt.itemMin, got, tt.want)
			}
		})
	}
}

func TestClampAIBetAmount(t *testing.T) {
	itemMin := 2.0
	floor := 2.0 // 1×
	def := 2.0   // 1×
	max := 6.0   // 3×

	// AI suggests 2× → keep
	if got := ClampAIBetAmount(4, itemMin, 100, 0, 100, true); got != 4 {
		t.Fatalf("want 4 got %.2f", got)
	}
	// Above 3× → clamp to 6
	if got := ClampAIBetAmount(99, itemMin, 100, 0, 100, true); got != max {
		t.Fatalf("want 3x=%.0f got %.2f", max, got)
	}
	// Below 1× floor → lift to 1×
	if got := ClampAIBetAmount(0.5, itemMin, 100, 0, 100, true); got != floor {
		t.Fatalf("want floor %.0f got %.2f", floor, got)
	}
	// Missing AI amount → default 1×
	if got := ClampAIBetAmount(0, itemMin, 100, 0, 100, true); got != def {
		t.Fatalf("want default %.0f got %.2f", def, got)
	}
	// 1× AI request → keep 1×
	if got := ClampAIBetAmount(itemMin, itemMin, 100, 0, 100, true); got != floor {
		t.Fatalf("want 1x got %.2f", got)
	}
	// Cash fraction: 12% of $50 = $6 → still 3× max
	if got := ClampAIBetAmount(99, itemMin, 50, 0, 50, true); got != max {
		t.Fatalf("want max %.0f got %.2f", max, got)
	}
	// Cash fraction: 12% of $40 = $4.8 binds before 3×
	if got := ClampAIBetAmount(99, itemMin, 40, 0, 40, true); got != 4.8 {
		t.Fatalf("want fraction 4.8 got %.2f", got)
	}
}

func TestItemMinBetAmount(t *testing.T) {
	if got := ItemMinBetAmount(0.5, 5, true); got != 2.5 {
		t.Fatalf("live min shares: got %v want 2.5", got)
	}
	if got := ItemMinBetAmount(0.1, 5, true); got != 1.0 {
		t.Fatalf("live $1 floor: got %v want 1.0", got)
	}
	if got := ItemMinBetAmount(0.2, 0, false); got != 1.0 {
		t.Fatalf("default 5 shares paper: got %v want 1.0", got)
	}
}
