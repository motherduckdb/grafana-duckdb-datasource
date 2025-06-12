# Changelog


## 0.2.1
- Fix multiple query variables.
- Build the plugin for intel mac.

## 0.2.0

- Bump duckdb to 1.2.2.
- Remove the dependency on the forked grafana-plugin-sdk-go.
- Avoid setting motherduck token as environment variable, use the duckdb config instead. 

## 0.1.1 
Changes default duckdb_directory to $GF_PATHS_DATA when the variable is set, so there's no permission issues with the Grafana (Ubuntu) docker image out of the box. 


## 0.1.0 

Initial release.
