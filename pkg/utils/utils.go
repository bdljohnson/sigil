package utils

import (
	"fmt"
	"strings"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
)

// StartAWSSession will return AWS Session
func StartAWSSession(region, profile, mfa string) *session.Session {
	options := session.Options{
		SharedConfigState:       session.SharedConfigEnable,
		AssumeRoleTokenProvider: awsMFATokenProvider(mfa),
	}
	if profile != "" {
		options.Profile = profile
	}
	awsConfig := aws.NewConfig()
	if region != "" {
		awsConfig.Region = &region
	}
	options.Config = *awsConfig
	sess := session.Must(session.NewSessionWithOptions(options))
	return sess
}

// GetInstance will return EC2 Instance's ID
func GetInstance(sess *session.Session, targetType string, target string) (*ec2.Instance, error) {
	if target == "" {
		err := fmt.Errorf("Specify the target")
		log.WithFields(log.Fields{
			"target": target,
		}).Error(err)
		return nil, err
	}
	switch targetType {	
	case "instance-id":
		instance, err := getFirstInstance(sess, &ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("instance-id"),
					Values: []*string{&target},
				},
			},
		})
		if err != nil {
			return nil, err
		}
		if instance == nil {
			return nil, fmt.Errorf("no instance with an instance id: %s", target)
		}
		return instance, err
	case "private-dns":
		instance, err := getFirstInstance(sess, &ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("private-dns-name"),
					Values: []*string{&target},
				},
				{
					Name:	aws.String("instance-state-name"),
					Values: []*string{aws.String("running")},
				},
			},
		})
		if err != nil {
			return nil, err
		}
		if instance == nil {
			return nil, fmt.Errorf("no instance with a private dns name: %s", target)
		}
		return instance, err
	case "name-tag":
		filters := []*ec2.Filter{}
		filters = append(filters, &ec2.Filter{
				Name:   aws.String("tag:Name"),
				Values: []*string{&target},
			})
		filters = append(filters, &ec2.Filter{
			Name:	aws.String("instance-state-name"),
			Values: []*string{aws.String("running")},
		})

		instance, err := getFirstInstance(sess, &ec2.DescribeInstancesInput{
			Filters: filters,
		})
		if err != nil {
			return nil, err
		}
		if instance == nil {
			return nil, fmt.Errorf("no instance with name tag: %s", target)
		}
		return instance, err
	case "tags":

		filters := []*ec2.Filter{}

		filters = append(filters, &ec2.Filter{
			Name:	aws.String("instance-state-name"),
			Values: []*string{aws.String("running")},
		})	

		// Create tag filters
		s := strings.Split(target, ",")
		for _, str := range s {
			tags := strings.Split(str, ":")
			filters = append(filters, &ec2.Filter{
				Name:	aws.String("tag:" + tags[0]),
				Values: []*string{&tags[1]},
			})
		}

		instance, err := getFirstInstance(sess, &ec2.DescribeInstancesInput{
			Filters: filters,
		})		

		if err != nil {
			return nil, err
		}
		if instance == nil {
			return nil, fmt.Errorf("no instance with name tag: %s", target)
		}
		return instance, err
	}

	return nil, fmt.Errorf("Unsupported target type: %s", target)
}

// Helper functions

func awsMFATokenProvider(token string) func() (string, error) {
	log.WithFields(log.Fields{
		"token": token,
	}).Debug("Get MFA Token Provider")
	if token == "" {
		return stscreds.StdinTokenProvider
	}
	return func() (string, error) {
		return token, nil
	}
}

func getFirstInstance(sess *session.Session, input *ec2.DescribeInstancesInput) (*ec2.Instance, error) {
	var target *ec2.Instance
	ec2Client := ec2.New(sess)
	err := ec2Client.DescribeInstancesPages(input,
		func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
			for _, reservation := range page.Reservations {
				for _, instance := range reservation.Instances {
					target = instance
					// Escape the function
					return false
				}
			}
			return !lastPage
		})
	if err != nil {
		return nil, err
	}
	return target, nil
}
