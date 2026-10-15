<script setup>
import { onMounted } from 'vue'
import SwaggerUI from 'swagger-ui-dist/swagger-ui-es-bundle.js'
import 'swagger-ui-dist/swagger-ui.css'

onMounted(() => {
  SwaggerUI({
    url: `${window.location.origin}/api/v1/external/openapi.yaml`,
    dom_id: '#swagger',
    persistAuthorization: true,
  })
})
</script>

<template>
  <div id="swagger" class="min-h-screen bg-white" />
</template>
