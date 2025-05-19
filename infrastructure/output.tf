# Outputs from a successful deployment allow showing resource properties to stdout
# Especially useful for parameterized/templated deployments that use variables

output "lambda_function_arn" {
  value = aws_lambda_function.lambda.invoke_arn
}

output "lambda_function_name" {
  value = aws_lambda_function.lambda.function_name
}

output "lambda_function_url" {
  value = aws_lambda_function_url.lambda.function_url
}
