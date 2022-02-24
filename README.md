# backupsmanager

Go project that exposes endpoints to trigger CockroachDB backups, and post them to GCP GS if configured

examples:

```
curl http://localhost:31000/crdbBackup/common-api-dev

curl http://localhost:31000/listBackups/

curl http://localhost:31000/fromBucket/common-api-dev_2022_01_04-101602.98
```

Cockroach user:

```
CREATE USER backups_manager;

GRANT admin TO backups_manager;
```

Cockroach restore command using CRDB sql client:

```
RESTORE DATABASE pg_commonapi FROM 'http://192.168.64.1.nip.io:31000/backups/common-api-dev/2022/01/24-163045.99';
```