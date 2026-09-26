$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$program = $env:KOKOROBOX_CORE_FIREWALL_PATH
if ([string]::IsNullOrWhiteSpace($program) -or -not [IO.Path]::IsPathRooted($program)) {
  throw 'Missing absolute core executable path'
}
$program = [IO.Path]::GetFullPath($program)
$group = 'KokoroBox Service Core'
$policy = New-Object -ComObject HNetCfg.FwPolicy2
if ([int]$policy.LocalPolicyModifyState -ne 0) {
  throw 'Windows Firewall policy prevents local inbound exceptions; check administrator policy'
}
foreach ($spec in @(
  @{ Name = 'KokoroBox Service Core TCP'; Protocol = 6 },
  @{ Name = 'KokoroBox Service Core UDP'; Protocol = 17 }
)) {
  $matchingRules = @($policy.Rules | Where-Object { $_.Name -eq $spec.Name })
  if ($matchingRules.Count -gt 1) { throw "Duplicate reserved core firewall rule '$($spec.Name)'" }
  $rule = if ($matchingRules.Count -eq 1) { $matchingRules[0] } else { $null }
  if ($null -ne $rule -and $rule.Grouping -ne $group) {
    throw "A firewall rule with the reserved name '$($spec.Name)' belongs to another group"
  }
  $isNew = $null -eq $rule
  if ($isNew) { $rule = New-Object -ComObject HNetCfg.FWRule }
  # Fixed rule names update the previous hash path in place after core upgrades.
  $rule.Name = $spec.Name
  $rule.Grouping = $group
  $rule.Description = 'Inbound proxy connections to the service-managed KokoroBox core. Mihomo controls LAN access.'
  $rule.ApplicationName = $program
  # Leave ServiceName unset: assigning even an empty string makes Rules.Add
  # reject the COM rule with E_INVALIDARG. The staged executable owns traffic.
  $rule.Protocol = $spec.Protocol
  $rule.LocalPorts = '*'
  $rule.RemotePorts = '*'
  $rule.LocalAddresses = '*'
  $rule.RemoteAddresses = '*'
  $rule.Direction = 1
  $rule.Action = 1
  $rule.Profiles = 2147483647
  $rule.InterfaceTypes = 'All'
  $rule.EdgeTraversal = $false
  $rule.Enabled = $true
  if ($isNew) { $null = $policy.Rules.Add($rule) }
  $actual = @($policy.Rules | Where-Object { $_.Name -eq $spec.Name })[0]
  if ($actual.ApplicationName -ine $program -or -not $actual.Enabled -or
      $actual.Direction -ne 1 -or $actual.Action -ne 1 -or
      $actual.Protocol -ne $spec.Protocol -or $actual.Profiles -ne 2147483647 -or
      -not [string]::IsNullOrEmpty($actual.ServiceName)) {
    throw "Failed to verify core firewall rule '$($spec.Name)'"
  }
}
