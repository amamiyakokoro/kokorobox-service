//go:build windows

package firewall

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run the production policy against an in-memory store with real, unregistered COM rules.
// No test changes the host firewall, and no elevated test process is required.
func TestPolicyCreatesUpdatesAndVerifiesStagedCoreRules(t *testing.T) {
	harness := `
$ErrorActionPreference = 'Stop'
$global:testPolicy = [pscustomobject]@{
  LocalPolicyModifyState = 0
  Rules = [System.Collections.ArrayList]::new()
}
function New-Object {
  param([string]$ComObject)
  if ($ComObject -eq 'HNetCfg.FwPolicy2') { return $global:testPolicy }
  if ($ComObject -ne 'HNetCfg.FWRule') { throw 'Unexpected COM object' }
  # Validate real COM property setters without adding rules to the host policy.
  return Microsoft.PowerShell.Utility\New-Object -ComObject HNetCfg.FWRule
}
function Assert-Rule($rule, $path) {
 if ($rule.ApplicationName -ine $path -or -not $rule.Enabled -or $rule.Action -ne 1 -or
     $rule.Direction -ne 1 -or $rule.Profiles -ne 2147483647 -or $rule.EdgeTraversal -or
     $rule.Grouping -ne 'KokoroBox Service Core') { throw ("Incorrect rule for '$path': " + ($rule | Select-Object ApplicationName,Enabled,Action,Direction,Profiles,EdgeTraversal,Grouping | ConvertTo-Json -Compress)) }
}
$policyBody = [IO.File]::ReadAllText($env:KOKOROBOX_TEST_POLICY)
$run = [scriptblock]::Create($policyBody)
$env:KOKOROBOX_CORE_FIREWALL_PATH = "C:ProgramDataKokoroBoxcore-runtimeoldhashmihomo.exe"
& $run
if ($global:testPolicy.Rules.Count -ne 2) { throw 'Missing protocol rules' }
foreach ($rule in $global:testPolicy.Rules) { Assert-Rule $rule $env:KOKOROBOX_CORE_FIREWALL_PATH }
if ($global:testPolicy.Rules[0].Protocol -ne 6 -or $global:testPolicy.Rules[1].Protocol -ne 17) { throw 'Incorrect protocols' }
& $run
if ($global:testPolicy.Rules.Count -ne 2) { throw 'Duplicate rules on restart' }
# A staged hash update must repair both protocols without accumulating rules.
$env:KOKOROBOX_CORE_FIREWALL_PATH = "C:ProgramDataKokoroBoxcore-runtime
ew hash's foldermihomo.exe"
$global:testPolicy.Rules[0].Enabled = $false
$global:testPolicy.Rules[0].Action = 0
& $run
if ($global:testPolicy.Rules.Count -ne 2) { throw 'Duplicate rules after update' }
foreach ($rule in $global:testPolicy.Rules) { Assert-Rule $rule $env:KOKOROBOX_CORE_FIREWALL_PATH }
# Never overwrite unrelated policy or pretend a centrally blocked policy was repaired.
$global:testPolicy.Rules[0].Grouping = 'Another app'
$failed = $false
try { & $run } catch { $failed = $true }
if (-not $failed) { throw 'Overwrote unrelated firewall rule' }
$global:testPolicy.LocalPolicyModifyState = 1
$failed = $false
try { & $run } catch { $failed = $true }
if (-not $failed) { throw 'Ignored administrator policy' }
`
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.ps1")
	if err := os.WriteFile(policyPath, []byte(policyScript), 0600); err != nil {
		t.Fatal(err)
	}
	testPath := filepath.Join(dir, "test.ps1")
	if err := os.WriteFile(testPath, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", testPath)
	command.Env = append(os.Environ(), "KOKOROBOX_TEST_POLICY="+policyPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("policy failed: %v\n%s", err, output)
	}
}

func TestEnsureRejectsInvalidExecutableBeforeTouchingFirewall(t *testing.T) {
	for _, path := range []string{"mihomo.exe", `C:\core\not-an-executable.txt`, "C:\\core\\mihomo.exe\x00other"} {
		if err := Ensure(path); err == nil {
			t.Fatalf("accepted invalid path %q", path)
		}
	}
	dir := filepath.Join(t.TempDir(), "mihomo.exe")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(dir); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("accepted directory: %v", err)
	}
}
