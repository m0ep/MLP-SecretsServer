# Build a Secrets Sharing Web Application

My code repository for the Manning LiveProject *Build a Secrets Sharing Web Application*

## Shell commands to make API calls
* Add a secret
    ```bash
    $: curl -X POST localhost:3000/ -d '{"plain_text":"mysecretpassword"}'
    {"id":"4cab2a2db6a3c31b01d804def28276e6"}
    ```
* Get a secret
    ```bash
    $: curl localhost:3000/4cab2a2db6a3c31b01d804def28276e6
    {"data":"mysecretpassword"}%
    ```