<#
.SYNOPSIS
    Provisions a Windows Server 2022 domain controller with an Enterprise CA and
    deliberately vulnerable certificate templates, for Aether Stage 47 (Batch 3)
    live qualification of AD CS detection, ESC abuse and PKINIT.

.DESCRIPTION
    Aether's Stage 47 cannot be live-qualified without a real CA. The Samba4 lab
    used for Batches 1 and 2 provisions an empty "CN=Public Key Services"
    container but no CA, no templates, and not even the pKICertificateTemplate
    schema class. This script builds the qualification environment on a real
    Windows domain.

    It installs the AD CS role, then creates three templates that each expose one
    ESC class, and configures the CA so ESC6 is also reachable. The templates are
    created vulnerable ON PURPOSE. Run this only on an isolated evaluation
    domain that contains no real identities or data.

    Requires the PSPKI PowerShell module for template publication:
        Install-Module PSPKI -Scope AllUsers
    Publication of a certificate template is not something the base AD cmdlets
    can do; PSPKI is the supported route. If the module is missing this script
    fails fast rather than half-configuring the CA.

.PARAMETER DomainFQDN
    The DNS name of the domain to promote, e.g. "adcs-lab.test".

.PARAMETER DomainAdminPassword
    Password for the domain Administrator account.

.PARAMETER SafeMode
    Skip template creation and CA flag changes. Installs a hardened CA only.
    Useful to bring up a baseline CA before experimenting.

.EXAMPLE
    .\adcs-windows-setup.ps1 -DomainFQDN adcs-lab.test -DomainAdminPassword 'Passw0rd123!'

.NOTES
    After this completes, record the CA's hostname. It is the value Aether
    requires in both --confirm-ca and the engagement file's authorized_cas.
#>

#Requires -RunAsAdministrator
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$DomainFQDN,

    [Parameter(Mandatory = $true)]
    [string]$DomainAdminPassword,

    [switch]$SafeMode
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# Constants. Flag values are taken from Microsoft's CA configuration schema,
# where the "Flags" value under
#   HKLM\SYSTEM\CurrentControlSet\Services\CertSvc\Configuration\<CA>\Flags
# is a bitfield.
#
# Note on a common documentation error: ESC6 is EDITF_ATTRIBUTESUBJECTALTNAME2
# = 0x00040000. Some write-ups also list 0x00000004 for this; 0x00000004 is not
# an attribute-SAN bit. Aether detects 0x00040000.
# ---------------------------------------------------------------------------
$EDITF_ATTRIBUTESUBJECTALTNAME2 = 0x00040000  # ESC6
$EDITF_ENABLEKEYREUSE          = 0x00000040

# Template attribute bitfields.
$CT_FLAG_ENROLLEE_SUPPLIES_SUBJECT = 0x00000001  # msPKI-Certificate-Name-Flag
$CT_FLAG_NO_SECURITY_EXTENSION      = 0x00080000  # msPKI-Enrollment-Flag

# EKUs.
$EKU_CLIENT_AUTH = '1.3.6.1.5.5.7.3.2'   # id-kp-clientAuth
$EKU_ANY_PURPOSE = '2.5.29.37.0'        # anyExtendedKeyUsage (ESC2)
$EKU_CERT_REQUEST_AGENT = '1.3.6.1.4.1.311.20.2.1'  # Certificate Request Agent (ESC3)

$ConfigNC = "CN=Configuration,DC=$((($DomainFQDN -split '\.') | ForEach-Object { "DC=$_" }) -join ',')"
$TemplateOU = "CN=Certificate Templates,CN=Public Key Services,CN=Services,$ConfigNC"

function Write-Step {
    param([string]$Message)
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Assert-Pspki {
    if (-not (Get-Module -ListAvailable -Name PSPKI)) {
        throw @"
The PSPKI module is required to create and publish certificate templates.

    Install-Module PSPKI -Scope AllUsers

Base AD cmdlets can read templates but cannot publish them to a CA, which is the
step that makes a template enrollable. Failing here rather than continuing keeps
the CA from ending up in a half-configured state.
"@
    }
    Import-Module PSPKI -ErrorAction Stop
}

function Assert-Domain {
    $domain = Get-ADDomain -ErrorAction SilentlyContinue
    if (-not $domain) {
        throw "This host is not a domain controller. Promote it to a domain controller with AD DS first."
    }
    if ($domain.DNSRoot -ne $DomainFQDN) {
        throw "This host is a DC for '$($domain.DNSRoot)' but -DomainFQDN is '$DomainFQDN'. They must match."
    }
    Write-Step "Domain confirmed: $($domain.DNSRoot)"
}

# ---------------------------------------------------------------------------
# 1. Install the CA
# ---------------------------------------------------------------------------
function Install-CertificateAuthority {
    Write-Step "Installing AD CS role features"
    $features = @('ADCS-Cert-Authority', 'ADCS-Web-Enrollment')
    foreach ($f in $features) {
        $state = (Get-WindowsFeature -Name $f -ErrorAction SilentlyContinue).InstallState
        if ($state -ne 'Installed') {
            Write-Host "    installing $f"
            Install-WindowsFeature -Name $f -IncludeManagementTools | Out-Null
        }
    }

    Write-Step "Installing the Enterprise CA"
    $existing = Get-ADCSertificationAuthority -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Host "    CA already present: $($existing.DisplayName)"
        return $existing
    }

    $ca = Install-ADCSertificationAuthority `
        -DomainName $DomainFQDN `
        -CAInstallMode Enterprise `
        -Credential (New-Object PSCredential('Administrator', (ConvertTo-SecureString $DomainAdminPassword -AsPlainText -Force))) `
        -Force

    Write-Step "CA installed: $($ca.DisplayName)"
    return $ca
}

# ---------------------------------------------------------------------------
# 2. CA flags (ESC6)
# ---------------------------------------------------------------------------
function Enable-Esc6CaFlag {
    <#
      Sets EDITF_ATTRIBUTESUBJECTALTNAME2 on the CA so that a request may carry
      an arbitrary subjectAltName regardless of what the template allows. This is
      the CA-level half of ESC6; the template half is handled per template.
    #>
    $ca = Get-ADCSertificationAuthority
    $key = "HKLM:\SYSTEM\CurrentControlSet\Services\CertSvc\Configuration\$($ca.CAName)"
    $name = 'Flags'
    $current = (Get-ItemProperty -Path $key -Name $name).$name
    $desired = $current -bor $EDITF_ATTRIBUTESUBJECTALTNAME2

    if ($current -eq $desired) {
        Write-Step "CA flag EDITF_ATTRIBUTESUBJECTALTNAME2 already set (0x{0:X8})" -f $current
        return
    }

    Write-Step "Setting CA flag EDITF_ATTRIBUTESUBJECTALTNAME2 (0x{0:X8} -> 0x{1:X8})" -f $current, $desired
    Set-ItemProperty -Path $key -Name $name -Value $desired
    Restart-Service -Name CertSvc -Force
}

# ---------------------------------------------------------------------------
# 3. Vulnerable templates
# ---------------------------------------------------------------------------
function New-VulnerableTemplate {
    <#
      Clones a stock template and applies one vulnerability. The clone keeps the
      stock extended key usages, so each result isolates exactly the condition
      named by its ESC class rather than mixing several.
    #>
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$DisplayName,
        [Parameter(Mandatory = $true)][ValidateSet('ESC1', 'ESC6', 'ESC15', 'ESC2', 'ESC3')][string]$Class,
        [string]$EKUs = $EKU_CLIENT_AUTH,
        [int]$SchemaVersion = 2
    )

    $existing = Get-ADObject -LDAPFilter "(&(objectClass=pKICertificateTemplate)(cn=$Name))" -SearchBase $TemplateOU -SearchScope OneLevel -Properties displayName -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Step "Template $Name already exists; leaving it alone"
        return $existing
    }

    Write-Step "Creating template $Name ($Class)"

    # Clone the stock User template, which already carries client-auth EKU and
    # a sane key size, then rewrite the properties that define the weakness.
    New-ADCertificateTemplate `
        -DisplayName $Name `
        -SourceTemplateName 'User' `
        -TemplateDisplayName $DisplayName

    $dn = "CN=$Name,$TemplateOU"
    $changes = @{
        'msPKI-Template-Schema-Version'    = $SchemaVersion
        'pKIExtendedKeyUsage'              = @($EKUs)
        'msPKI-Certificate-Application-Policy' = @($EKUs)
        'revision'                         = 100
    }

    switch ($Class) {
        'ESC1' {
            # ESC1: the enrollee supplies the subject, so the CA honours a SAN
            # carried in the request instead of deriving it from the requester.
            $changes['msPKI-Certificate-Name-Flag'] = $CT_FLAG_ENROLLEE_SUPPLIES_SUBJECT
            $changes['msPKI-Enrollment-Flag'] = 0
        }
        'ESC6' {
            # ESC6 template half: enrollee-supplied subject, no manager approval,
            # and the security extension cleared so the SAN is taken as sent.
            $changes['msPKI-Certificate-Name-Flag'] = $CT_FLAG_ENROLLEE_SUPPLIES_SUBJECT
            $changes['msPKI-Enrollment-Flag'] = $CT_FLAG_NO_SECURITY_EXTENSION
        }
        'ESC15' {
            # ESC15: a schema version 1 template. Version 1 templates do not
            # enforce the application policy recorded on the template, so the
            # request can add an EKU of the caller's choosing.
            $changes['msPKI-Template-Schema-Version'] = 1
            $changes['msPKI-Certificate-Name-Flag'] = $CT_FLAG_ENROLLEE_SUPPLIES_SUBJECT
        }
        'ESC2' {
            $changes['pKIExtendedKeyUsage'] = @($EKU_ANY_PURPOSE)
        }
        'ESC3' {
            $changes['pKIExtendedKeyUsage'] = @($EKU_CLIENT_AUTH, $EKU_CERT_REQUEST_AGENT)
        }
    }

    foreach ($attribute in $changes.Keys) {
        Set-ADObject -Identity $dn -Replace @{
            $attribute = $changes[$attribute]
        }
    }

    # Grant Domain Users enrolment. Without this the template is vulnerable in
    # principle but unusable, and Aether's ESC1 detection would correctly
    # report no finding, which is not what we want to qualify against.
    $domainUsers = Get-ADGroup -Identity 'Domain Users'
    $adcsPartition = Get-ADObject -SearchBase $ConfigNC -SearchScope Subtree `
        -LDAPFilter '(objectClass=container)(cn=Public Key Services)' -ResultSetSize 1
    Add-ADObjectAce -Identity $adcsPartition.DistinguishedName `
        -Type Allow -ObjectType 'msPKI-Enrollment-Certificates-Access' `
        -ActiveDirectoryRights ExtendedRight `
        -AccessControlProperty 'Enrollment' -InheritanceType All `
        -ObjectAceType 'ACE' -ObjectSecurityGroup $domainUsers

    Register-ADCSecurityTemplate -Name $Name
    Write-Step "Published $Name"
}

# ---------------------------------------------------------------------------
# 4. Verification
# ---------------------------------------------------------------------------
function Test-QualificationEnvironment {
    $ca = Get-ADCSertificationAuthority
    $templates = Get-ADObject -SearchBase $TemplateOU -SearchScope OneLevel `
        -Filter '(objectClass=pKICertificateTemplate)' `
        -Properties msPKI-Certificate-Name-Flag, msPKI-Enrollment-Flag, `
                  msPKI-Template-Schema-Version, pKIExtendedKeyUsage, `
                  msPKI-Cert-Template-OID, displayName

    $caFlags = (Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\CertSvc\Configuration\$($ca.CAName)" -Name 'Flags').Flags

    [pscustomobject]@{
        CAName                 = $ca.CAName
        CAHostname             = $ca.HostComputer
        CAConfigurationFlags  = ('0x{0:X8}' -f $caFlags)
        Esc6CaFlagPresent      = [bool]($caFlags -band $EDITF_ATTRIBUTESUBJECTALTNAME2)
        TemplateCount          = @($templates).Count
        Templates              = ($templates | ForEach-Object {
            '{0} (nameFlag=0x{1:X}, enrollFlag=0x{2:X}, schema={3}, eku={4})' -f `
                $_.displayName, `
                [int]$_.'msPKI-Certificate-Name-Flag', `
                [int]$_.'msPKI-Enrollment-Flag', `
                $_.'msPKI-Template-Schema-Version', `
                ($_.'pKIExtendedKeyUsage' -join ',')
        }) -join "`n    "
    }
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
Assert-Pspki
Assert-Domain
$ca = Install-CertificateAuthority

if ($SafeMode) {
    Write-Step 'SafeMode: skipping vulnerable template creation and CA flag changes.'
    Write-Step 'The CA is hardened and is NOT suitable for ESC qualification.'
} else {
    Enable-Esc6CaFlag

    New-VulnerableTemplate -Name 'ESC1-Test'  -DisplayName 'ESC1 Test'  -Class 'ESC1'  -EKUs $EKU_CLIENT_AUTH
    New-VulnerableTemplate -Name 'ESC6-Test'  -DisplayName 'ESC6 Test'  -Class 'ESC6'  -EKUs $EKU_CLIENT_AUTH
    New-VulnerableTemplate -Name 'ESC15-Test' -DisplayName 'ESC15 Test' -Class 'ESC15' -EKUs $EKU_CLIENT_AUTH -SchemaVersion 1
    New-VulnerableTemplate -Name 'ESC2-Test'  -DisplayName 'ESC2 Test'  -Class 'ESC2'  -EKUs $EKU_ANY_PURPOSE
    New-VulnerableTemplate -Name 'ESC3-Test'  -DisplayName 'ESC3 Test'  -Class 'ESC3'  -EKUs $EKU_CLIENT_AUTH
}

Write-Step 'Environment state'
Test-QualificationEnvironment | Format-List

Write-Step 'Done.'
Write-Host @"

Next steps for Aether Stage 47 qualification:

  1. Record the CA hostname shown above. It is required in two places:
       - aether ad cert ... --confirm-ca <CAHostname>
       - the engagement file's "authorized_cas" array

  2. The KDC must trust the issuing CA for PKINIT to work. On the issuing CA,
     add the domain's NTAuthCertificates publication for the KDC, or enroll the
     KDC with a cert from a CA in the KDC's trust store. Without this, PKINIT
     against a certificate issued here will fail chain validation and that
     failure is a harness problem, not an Aether defect.

  3. Do not point this at any domain containing real identities. The templates
     created here are intentionally exploitable.
"@ -ForegroundColor Yellow
