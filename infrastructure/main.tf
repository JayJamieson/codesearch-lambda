# Configures terraform with resource providers that are used to create and manage resources
# The AWS provider is used to create and manage AWS resources
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.84.0"
    }
  }
}

# Configuration of the AWS provider such as region, access key, secret key
# See https://registry.terraform.io/providers/hashicorp/aws/5.84.0/docs
# Credentials are typically read from environment variables, however
# when running in CI/CD pipelines, credentials are often passed in as variables.
#
# The current configuration assumes credentials are configured in environment variables
# or in the ~/.aws/credentials file
provider "aws" {
  region = var.region
}

# Helper to get the current AWS account ID being used
# Gives access to account_id, user_id, and ARN
data "aws_caller_identity" "current" {}

data "archive_file" "lambda" {
  type        = "zip"
  source_file  = "${path.module}/bootstrap"
  output_path = "${path.module}/function.zip"
}

# Creates our Lambda function
resource "aws_lambda_function" "lambda" {
  function_name = "codesearch"

  # Our previously created IAM role allowing execution and logging access
  role          = aws_iam_role.lambda_iam_role.arn

  # CPU architecture for the Lambda function
  # The default is x86_64, arm64 is also supported
  architectures = ["x86_64"]

  # Environment used to run handler code
  runtime = "provided.al2"

  # For runtimes like provided.al2, the handler is the name of the file
  # Other runtimes like python, nodejs, etc. the handler is the name of the file
  # and the function name separated by a dot
  # exmaple: nodejs index.handler
  handler = "bootstrap"

  filename = "${path.module}/function.zip"

  # helps terraform to detect changes in the Lambda function code
  source_code_hash = data.archive_file.lambda.output_base64sha256

  # maximum execution time for the Lambda function in seconds
  # The default is 3 seconds, and the maximum is 900 seconds (15 minutes)
  # Function is terminate if it does not complete within the timeout period
  timeout      = 900

  memory_size = 256

  depends_on = [
    aws_iam_role_policy_attachment.lambda_policy_attachment,
  ]
}

# Creates a Lambda function URL for the Lambda function
# This provides a simple entrypoint to invoke the Lambda function over HTTPS
# without needing to set up an API Gateway, Route53 and Certificate
# See https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/lambda_function_url
resource "aws_lambda_function_url" "lambda" {
  function_name      = aws_lambda_function.lambda.function_name
  authorization_type = "NONE"
}

# Creates the IAM role for the Lambda function
resource "aws_iam_role" "lambda_iam_role" {
  name               = "iam_codesearch_lambda"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
}

# Attaches an IAM policy to the Lambda function's execution role
#
# AWS Provides managed policy documents for reusability, saves duplicating basic permissions over and over again
# It allows the Lambda function to create log groups, log streams, and put log events
# This is needed for logging and monitoring the Lambda function's execution
# https://docs.aws.amazon.com/aws-managed-policy/latest/reference/about-managed-policy-reference.html
resource "aws_iam_role_policy_attachment" "lambda_policy_attachment" {
  role       = aws_iam_role.lambda_iam_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"

  # The policy_arn above has the following policy document

  # {
  #   "Version": "2012-10-17",
  #   "Statement": [
  #       {
  #           "Effect": "Allow",
  #           "Action": [
  #               "logs:CreateLogGroup",
  #               "logs:CreateLogStream",
  #               "logs:PutLogEvents"
  #           ],
  #           "Resource": "*"
  #       }
  #   ]
  # }
}

# Defines the IAM policy document for the Lambda function's execution role
# This policy allows the Lambda service to perform sts:AssumeRole action
# This creates a trust relationship with the Lambda service to assume the role
# https://docs.aws.amazon.com/lambda/latest/dg/lambda-intro-execution-role.html
data "aws_iam_policy_document" "assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}
