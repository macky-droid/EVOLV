#!/usr/bin/env ruby
# frozen_string_literal: true

# Workflow metadata audit. Runs in the `workflow / shell syntax` CI job.
#
# The rules are intentionally mechanical: they are the ones that keep a run
# finite and its supply chain reviewable.
#   - every workflow has explicit permissions and concurrency
#   - every job has a timeout-minutes
#   - every remote action is pinned to a version tag or a full commit SHA

require "yaml"

errors = []
files = Dir[".github/workflows/*.{yml,yaml}"].sort
abort "no workflow files found" if files.empty?

files.each do |file|
  begin
    wf = YAML.load_file(file)
  rescue StandardError => e
    errors << "#{file}: invalid YAML: #{e.message}"
    next
  end

  errors << "#{file}: missing top-level permissions" unless wf["permissions"]
  errors << "#{file}: missing concurrency" unless wf["concurrency"]

  jobs = wf["jobs"]
  unless jobs.is_a?(Hash) && !jobs.empty?
    errors << "#{file}: no jobs"
    next
  end

  jobs.each do |job_name, job|
    unless job.is_a?(Hash)
      errors << "#{file}: job #{job_name} is not a mapping"
      next
    end
    errors << "#{file}: job #{job_name} missing timeout-minutes" unless job["timeout-minutes"]

    checks = []
    checks << ["job #{job_name}", job["uses"]] if job["uses"]
    (job["steps"] || []).each_with_index do |step, index|
      checks << ["job #{job_name} step #{index + 1}", step["uses"]] if step.is_a?(Hash) && step["uses"]
    end

    checks.each do |where, uses|
      next if uses.start_with?("./")
      next if uses.match?(/@v\d+(\.\d+){0,2}\z/)
      next if uses.match?(/@[0-9a-f]{40}\z/)

      errors << "#{file}: #{where} is not pinned: #{uses}"
    end
  end
end

if errors.any?
  warn "workflow audit failed:"
  errors.each { |e| warn "  - #{e}" }
  exit 1
end

puts "workflow audit ok: #{files.size} files"
