import logging
import os
import tempfile
from datetime import datetime, timedelta

from django.core.management.base import BaseCommand
from django.db import connection

logger = logging.getLogger(__name__)


class Command(BaseCommand):
    help = "Archive old message partitions to S3 and detach from PostgreSQL"

    def add_arguments(self, parser):
        parser.add_argument(
            "--dry-run",
            action="store_true",
            help="Run without detaching partition or uploading",
        )

    def handle(self, *args, **options):
        self.dry_run = options["dry_run"]
        self.stdout.write(self.style.SUCCESS("Starting archive job..."))

        try:
            import boto3
        except ImportError:
            self.stdout.write(self.style.ERROR("boto3 not installed"))
            return

        today = datetime.today()
        first_day = today.replace(day=1)
        prev_month = first_day - timedelta(days=1)
        partition_name = f"messages_y{prev_month.year}m{prev_month.month:02d}"

        with connection.cursor() as cursor:
            cursor.execute(
                "SELECT EXISTS (SELECT FROM pg_tables WHERE tablename = %s)",
                [partition_name],
            )
            exists = cursor.fetchone()[0]
            if not exists:
                self.stdout.write(
                    self.style.WARNING(f"Partition {partition_name} not found. Nothing to archive.")
                )
                return

            self.stdout.write(f"Archiving partition: {partition_name}")

            if self.dry_run:
                self.stdout.write(self.style.NOTICE("Dry run - skipping actual archive"))
                return

            # Export to Parquet-like CSV first, then compress
            with tempfile.NamedTemporaryFile(suffix=".csv.gz", delete=False) as tmp:
                tmp_path = tmp.name

            cursor.execute(
                f"COPY (SELECT * FROM {partition_name}) TO PROGRAM 'gzip > {tmp_path}' WITH (FORMAT CSV, HEADER)"
            )

            # Upload to S3/MinIO
            s3_endpoint = os.getenv("S3_ENDPOINT", "http://minio:9000")
            s3_bucket = os.getenv("S3_ARCHIVE_BUCKET", "message-archive")
            s3_key = f"messages/{partition_name}.csv.gz"

            s3 = boto3.client(
                "s3",
                endpoint_url=s3_endpoint,
                aws_access_key_id=os.getenv("S3_ACCESS_KEY", "minioadmin"),
                aws_secret_access_key=os.getenv("S3_SECRET_KEY", "minioadmin"),
            )

            # Ensure bucket exists
            try:
                s3.head_bucket(Bucket=s3_bucket)
            except:
                s3.create_bucket(Bucket=s3_bucket)

            s3.upload_file(tmp_path, s3_bucket, s3_key)
            os.remove(tmp_path)

            # Detach partition
            cursor.execute(f"ALTER TABLE messages DETACH PARTITION {partition_name}")
            cursor.execute(f"DROP TABLE {partition_name}")

            self.stdout.write(
                self.style.SUCCESS(
                    f"Partition {partition_name} archived to s3://{s3_bucket}/{s3_key} and removed from DB"
                )
            )
