module github.com/stepro/kdo

go 1.12

require (
	github.com/docker/docker v20.10.7+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/moby/buildkit v0.9.0
	github.com/spf13/cobra v1.2.1
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c
)

replace github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.6.0
