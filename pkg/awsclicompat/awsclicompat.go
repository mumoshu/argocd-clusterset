package awsclicompat

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"os"
)

// NewSession creates a new AWS session for the given AWS region.
//
// The following credential sources are supported:
//
// 1. static credentials (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
// 2. static credentials loaded from profiles (AWS_PROFILE, when AWS_SDK_LOAD_CONFIG=true)
// 3. dynamic credentials obtained by assuming the role using static credentials loaded from the profile (AWS_PROFILE, when AWS_SDK_LOAD_CONFIG=true)
// 4. dynamic credentials obtained by assuming the role using static credentials loaded from the env (FORCE_AWS_PROFILE=true w/ credential_source=Environment)
//
// The fourth option of using FORCE_AWS_PROFILE=true and AWS_PROFILE=yourprofile is equivalent to `aws --profile ${AWS_PROFILE}`.
// See https://github.com/variantdev/vals/issues/19#issuecomment-600437486 for more details and why and when this is needed.
func NewSession(region, profile, roleARN string) *session.Session {
	var cfg *aws.Config
	if region != "" {
		cfg = aws.NewConfig().WithRegion(region)
	} else {
		cfg = aws.NewConfig()
	}

	opts := session.Options{
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
		SharedConfigState:       session.SharedConfigEnable,
		Config:                  *cfg,
		Profile:                 profile,
	}

	if os.Getenv("FORCE_AWS_PROFILE") == "true" {
		opts.Profile = os.Getenv("AWS_PROFILE")
	}

	sess := session.Must(session.NewSessionWithOptions(opts))

	if roleARN != "" {
		creds := stscreds.NewCredentialsWithClient(sts.New(sess), roleARN)
		return sess.Copy(&aws.Config{Credentials: creds})
	}

	return sess
}
