#!/usr/local/bin/python3

from collections import Counter
from cryptography import x509
from cryptography.hazmat.backends import default_backend
from publicsuffixlist import PublicSuffixList
import argparse
import boto3
import datetime
import os
import pkioracle
import sys
import time
import geoip2.database

parser = argparse.ArgumentParser()
parser.add_argument("--path", help="Path to folder on disk to store certs")
parser.add_argument("--s3bucket", help="S3 Bucket to store certs")
parser.add_argument("--psl", help="Path to effective_tld_names.dat")
parser.add_argument("--problems", default="problems", help="File to record errors")
parser.add_argument("--output", help="File to place the output report")
parser.add_argument("--geoipDb", help="Path to GeoIP2-City.mmdb")

# I/O
args = parser.parse_args()

psl = PublicSuffixList()
if args.psl:
  with open(args.psl, "rb") as f:
    psl = PublicSuffixList(f)

oracle = pkioracle.Oracle()
client = boto3.client('s3')
s3 = boto3.resource('s3')

if args.geoipDb:
  oracle.geoDB = geoip2.database.Reader(args.geoipDb)

counter = Counter()

def processDisk(path, errorFd):
  for root, _, files in os.walk(path):
    for file in files:
      file_path = os.path.join(root, file)
      try:
        with open(file_path, 'rb') as f:
          der_data = f.read()
          cert = x509.load_der_x509_certificate(der_data, default_backend())
          metaData = oracle.getMetadataForCert(psl, cert)
          oracle.processCertMetadata(metaData)
      except ValueError as e:
        errorFd.write("{}\t{}\n".format(file_path, e))

def processS3(bucket, errorFd):
  # response = client.list_objects_v2(
  #   Bucket=bucket,
  #   MaxKeys=1024,
  #   # StartAfter='string',
  # )

  # print(response)
  # for obj in response['Contents']:
  #   print(obj)

  for obj in s3.Bucket(bucket).objects.filter(Prefix="cert/"):
    # print(obj)
    parts = obj.key.split("/")
    year = int(parts[1])
    dayOfYear = int(parts[2])

    # Is this expired (check by looking the path so we don't have to continue
    # to load)
    now = time.gmtime()
    if (year < now.tm_year) or (year == now.tm_year and dayOfYear < now.tm_yday):
      counter["Expired"] += 1
      continue

    # OK, not expired yet!
    # Grab the metadata, because let's assume we've already processed the cert
    headObj = client.head_object(Bucket=obj.bucket_name, Key=obj.key)
    try:
      # print("Trying {}".format(headObj))
      oracle.processCertMetadata(headObj['Metadata'])

      counter["Metadata Up-to-Date"] += 1
    except KeyError as missingKey:
      # I guess we haven't processed the cert yet, so let's process it.
      dlObj = obj.get()
      der_data = dlObj['Body'].read()
      try:
        cert = x509.load_der_x509_certificate(der_data, default_backend())
        metaData = oracle.getMetadataForCert(psl, cert)
        # print("Updating metadata for {} to {}".format(obj.key, metaData))
        # Save back that metadata
        result = obj.copy_from(CopySource={'Bucket':obj.bucket_name, 'Key':obj.key},
                               Metadata=metaData, MetadataDirective="REPLACE")

        counter["Metadata Updated"] += 1
      except ValueError as e:
        # Problem parsing the certifiate
        errorFd.write("{}\t{}\t{}\n".format(obj.key, obj, e))
        counter["Certificate Parse Errors"] += 1

    counter["Total Certificates Processed"] += 1

# Main
with open(args.problems, "w+") as problemFd:
  if args.path:
    processDisk(args.path, problemFd)
  elif args.s3bucket:
    processS3(args.s3bucket, problemFd)
  else:
    parser.print_usage()
    sys.exit(0)

# Clean up the oracle and serialize it
serializedOracle = oracle.serialize()

# Either go to file, or to stdout
if args.output:
  with open(args.output, "w") as outFd:
    outFd.write(serializedOracle)
else:
  # Pretty print it. Cheat using json module
  import json
  parsed = json.loads(serializedOracle)
  print(json.dumps(parsed, indent=4))

print("Done. Process results: {}".format(counter))