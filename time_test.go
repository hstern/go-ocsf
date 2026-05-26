// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf

import (
	"testing"
	"time"
)

func TestTimeFromMillis_Epoch(t *testing.T) {
	got := TimeFromMillis(0)
	want := time.Unix(0, 0).UTC()
	if !got.Equal(want) {
		t.Errorf("TimeFromMillis(0) = %v, want %v", got, want)
	}
	if got.Location() != time.UTC {
		t.Errorf("TimeFromMillis(0).Location() = %v, want UTC", got.Location())
	}
}

func TestTimeFromMillis_KnownValue(t *testing.T) {
	// 1618524549901 ms = 2021-04-15T22:09:09.901Z (the example value
	// from the OCSF schema's timestamp_t description).
	got := TimeFromMillis(1618524549901)
	want := time.Date(2021, 4, 15, 22, 9, 9, 901_000_000, time.UTC)
	if !got.Equal(want) {
		t.Errorf("TimeFromMillis(1618524549901) = %v, want %v", got, want)
	}
}

func TestMillisFromTime_RoundTrip(t *testing.T) {
	values := []int64{0, 1, 1_000, 1_618_524_549_901, -1, -1_000_000}
	for _, ms := range values {
		gotMS := MillisFromTime(TimeFromMillis(ms))
		if gotMS != ms {
			t.Errorf("round-trip lost: TimeFromMillis(%d) -> MillisFromTime = %d", ms, gotMS)
		}
	}
}

func TestMillisFromTime_SubMillisecondTruncation(t *testing.T) {
	// 999_999ns within a second of the epoch is < 1ms, truncates
	// to 0 ms.
	got := MillisFromTime(time.Unix(0, 999_999))
	if got != 0 {
		t.Errorf("MillisFromTime(epoch + 999_999ns) = %d, want 0", got)
	}
	// 1_500_000 ns = 1.5 ms, truncates to 1 ms.
	got = MillisFromTime(time.Unix(0, 1_500_000))
	if got != 1 {
		t.Errorf("MillisFromTime(epoch + 1_500_000ns) = %d, want 1", got)
	}
}
