#!/bin/bash -e
systemctl daemon-reload
systemctl enable aerospike-backup-service
systemctl start aerospike-backup-service
