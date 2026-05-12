// Name collision: exercises the AWS provider's real name collisions
// between top-level resource names and nested block type names.
//
// AWS has natural collisions like:
//   - aws_s3_bucket nested "logging" → S3BucketLogging
//     aws_s3_bucket_logging standalone → S3BucketLogging
//
// If collision avoidance in the generator breaks, the generated code
// won't compile (duplicate type definitions) and this program fails to build.
package main

import (
	"fmt"
	"log"

	"github.com/DataDog/tforge/config"
	"github.com/DataDog/tforge/render"
	"github.com/DataDog/tforge/testdata/golden/name_collision/aws"
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
	if err := aws.NewS3BucketLogging(stack, "example",
		aws.S3BucketLogging_SetBucket("${aws_s3_bucket.main.id}"),
		aws.S3BucketLogging_SetTargetBucket("${aws_s3_bucket.logs.id}"),
		aws.S3BucketLogging_SetTargetPrefix("access-logs/"),
	); err != nil {
		log.Fatal(err)
	}

	// Resource whose nested "logging" type was skipped due to collision.
	if err := aws.NewS3Bucket(stack, "main",
		aws.S3Bucket_SetBucket("my-app-bucket"),
		aws.S3Bucket_SetTags(map[string]string{"Environment": "production"}),
	); err != nil {
		log.Fatal(err)
	}

	// Log target bucket.
	if err := aws.NewS3Bucket(stack, "logs",
		aws.S3Bucket_SetBucket("my-app-logs"),
	); err != nil {
		log.Fatal(err)
	}

	data, err := render.ToJSON(stack)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}
