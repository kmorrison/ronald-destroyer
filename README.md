To start database
```
docker run --name local-pg2 -e POSTGRES_PASSWORD=<your password here> -p 5432 -d postgres
```

To create database
```
docker exec -it <container id from above> psql --user=postgres --password -c "CREATE DATABASE ronny"
```

To migrate database
```
make build
./bin/migrate
```

To run bot
```
make bot
```