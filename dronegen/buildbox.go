package main

import "fmt"

func buildboxPipelineSteps() []step {
	steps := []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Commands: []string{
				`git clone --depth 1 --single-branch --branch ${DRONE_SOURCE_BRANCH:-master} https://github.com/gravitational/${DRONE_REPO_NAME}.git .`,
				`git checkout ${DRONE_COMMIT}`,
			},
		},
		waitForDockerStep(),
	}

	for _, name := range []string{"buildbox", "buildbox-centos6", "buildbox-arm"} {
		for _, fips := range []bool{false, true} {
			// FIPS is unsupported on ARM/ARM64
			if name == "buildbox-arm" && fips {
				continue
			}
			steps = append(steps, buildboxPipelineStep(name, fips))
		}
	}
	return steps
}

func buildboxPipelineStep(buildboxName string, fips bool) step {
	if fips {
		buildboxName += "-fips"
	}
	return step{
		Name:  buildboxName,
		Image: "docker",
		Environment: map[string]value{
			"QUAYIO_DOCKER_USERNAME": {fromSecret: "QUAYIO_DOCKER_USERNAME"},
			"QUAYIO_DOCKER_PASSWORD": {fromSecret: "QUAYIO_DOCKER_PASSWORD"},
		},
		// Buildbox builds run sequentially, so any failure of an earlier step will prevent later steps from running.
		// The CentOS 6 FIPS buildbox is currently failing to build, so we ignore this to prevent pushes to
		// master from being unnecessarily marked as failures. The underlying issue will be fixed soon.
		Failure: "ignore",
		Volumes: dockerVolumeRefs(),
		Commands: []string{
			`apk add --no-cache make`,
			`chown -R $UID:$GID /go`,
			`docker login -u="$$QUAYIO_DOCKER_USERNAME" -p="$$QUAYIO_DOCKER_PASSWORD" quay.io`,
			fmt.Sprintf(`make -C build.assets %s`, buildboxName),
			fmt.Sprintf(`docker push quay.io/gravitational/teleport-%s:$RUNTIME`, buildboxName),
		},
	}
}

func buildboxPipeline() pipeline {
	p := newKubePipeline("build-buildboxes")
	p.Environment = map[string]value{
		"RUNTIME": goRuntime,
		"UID":     {raw: "1000"},
		"GID":     {raw: "1000"},
	}
	p.Trigger = triggerPushMasterOnly
	p.Workspace = workspace{Path: "/go/src/github.com/gravitational/teleport"}
	p.Volumes = dockerVolumes()
	p.Services = []service{
		dockerService(),
	}
	p.Steps = buildboxPipelineSteps()
	return p
}
