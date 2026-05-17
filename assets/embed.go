// Package assets embeda recursos estáticos via //go:embed.
package assets

import _ "embed"

//go:embed common_domains.txt
var CommonDomains string

//go:embed free_providers.txt
var FreeProviders string

//go:embed role_prefixes.txt
var RolePrefixes string

//go:embed disposable_domains.txt
var DisposableDomains string

//go:embed tlds.txt
var TLDs string
