package projectcfg

// AllTypes lists supported AP_PACK_PROJECT.project_type values.
var AllTypes = []string{"maven", "npm", "vue", "go", "gradle", "custom"}

// DeployEnvTypes are valid env_type values for deploy_env_rec_id.
var DeployEnvTypes = map[string]bool{
	"ssh":  true,
	"sftp": true,
	"ftp":  true,
}

func IsValidType(t string) bool {
	for _, v := range AllTypes {
		if v == t {
			return true
		}
	}
	return false
}
