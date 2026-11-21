package sep1

import (
	"testing"
)

func TestParseAccounts(t *testing.T) {
	t.Parallel()

	valid := "GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG"

	cases := []struct {
		name          string
		body          string
		wantLen       int
		wantMalformed int
		wantNil       bool
		wantStep      Step
	}{
		{
			name:          "accounts absent",
			body:          "",
			wantLen:       0,
			wantMalformed: 0,
			wantNil:       true,
			wantStep:      StepNoKeys,
		},
		{
			name:          "accounts empty",
			body:          "ACCOUNTS = []",
			wantLen:       0,
			wantMalformed: 0,
			wantNil:       false,
			wantStep:      StepOK,
		},
		{
			name:          "valid beside non-string",
			body:          "ACCOUNTS = [\"" + valid + "\", 42]",
			wantLen:       1,
			wantMalformed: 1,
			wantNil:       false,
			wantStep:      StepOK,
		},
		{
			name:          "all invalid",
			body:          "ACCOUNTS = [42, 7]",
			wantLen:       0,
			wantMalformed: 2,
			wantNil:       false,
			wantStep:      StepOK,
		},
		{
			name:          "all invalid beside passphrase",
			body:          "NETWORK_PASSPHRASE = \"x\"\nACCOUNTS = [42, 7]",
			wantLen:       0,
			wantMalformed: 2,
			wantNil:       false,
			wantStep:      StepOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			id, step, _ := Parse([]byte(tc.body))
			if len(id.Accounts) != tc.wantLen {
				t.Fatalf("len(Accounts) = %d, want %d", len(id.Accounts), tc.wantLen)
			}
			if id.MalformedAccounts != tc.wantMalformed {
				t.Fatalf("MalformedAccounts = %d, want %d", id.MalformedAccounts, tc.wantMalformed)
			}
			if (id.Accounts == nil) != tc.wantNil {
				t.Fatalf("Accounts nil = %v, want %v", id.Accounts == nil, tc.wantNil)
			}
			if step != tc.wantStep {
				t.Fatalf("step = %v, want %v", step, tc.wantStep)
			}
		})
	}
}
