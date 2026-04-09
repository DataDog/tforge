// Name collision: exercises the AWS provider's real name collisions
// between top-level resource names and nested block type names.
//
// AWS has natural collisions like:
//   - aws_s3_bucket nested "logging" → AwsS3BucketLogging
//     aws_s3_bucket_logging standalone → AwsS3BucketLogging
//
// If collision avoidance in the generator breaks, the generated code
// won't compile (duplicate type definitions) and this program fails to build.
package main

import (
	"fmt"
	"log"

	"tforge/config"
	"tforge/render"
	"tforge/testdata/golden/name_collision/aws"
)

func main() {
	stack := config.NewStack("name-collision-golden")

	stack.AddProvider(config.Provider{
		Name:    "aws",
		Source:  "hashicorp/aws",
		Version: "~> 5.100",
		Config:  map[string]any{"region": "us-east-1"},
	})

	// Standalone resource that won the collision with aws_s3_bucket's nested "logging".
	if err := aws.NewAwsS3BucketLogging(stack, "example", aws.AwsS3BucketLogging{
		Bucket:       "${aws_s3_bucket.main.id}",
		TargetBucket: "${aws_s3_bucket.logs.id}",
		TargetPrefix: "access-logs/",
	}); err != nil {
		log.Fatal(err)
	}

	// Resource whose nested "logging" type was skipped due to collision.
	if err := aws.NewAwsS3Bucket(stack, "main", aws.AwsS3Bucket{
		Bucket: "my-app-bucket",
		Tags:   map[string]string{"Environment": "production"},
	}); err != nil {
		log.Fatal(err)
	}

	// Log target bucket.
	if err := aws.NewAwsS3Bucket(stack, "logs", aws.AwsS3Bucket{
		Bucket: "my-app-logs",
	}); err != nil {
		log.Fatal(err)
	}

	data, err := render.ToJSON(stack)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}
