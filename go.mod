module github.com/test-network-function/l2discovery

go 1.18

replace github.com/test-network-function/l2discovery/l2lib => ./l2lib

replace github.com/test-network-function/l2discovery/l2lib/pkg/export => ./l2lib/pkg/export

require (
	github.com/sirupsen/logrus v1.9.0
	github.com/test-network-function/l2discovery/l2lib/pkg/export v0.0.0-00010101000000-000000000000
)

require (
	golang.org/x/sys v0.0.0-20220722155257-8c9f86f7a55f // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
