# Auditoria — hotfix 13 / issue #8

Os relatórios anexados à issue #8 são da prévia 12: ABI esperado/real 9, 121 ossos locais, nenhum recebido/aplicado no remoto e estado idle. Um cliente capturou 141 pedidos de efeitos e nenhum foi aplicado. Isso indica que a atualização nativa estava instalada, mas não comprova o desenho de partículas. Os relatórios privados não foram incorporados ao repositório.

A causa reproduzida está no launcher: ControlStream aumentava o limite para 65.535 caracteres apenas em hm_native_remote; hm_native_sample ainda recebia o limite padrão de 256. A prévia 12 passou a usar o segundo comando para transmitir a mesma pose completa. Sua recusa acontece antes de abrir/escrever o socket local e o Bridge retorna ao estado de espera, impedindo atualização contínua.

Correção restrita à lista dos comandos que podem transportar quadros longos. Ambos usam o mesmo teto de 65.535, com rejeição de excesso e CR/LF/NUL; comandos não contemplados mantêm seus limites. ControlClient de ações pontuais não é ampliado.

A regressão nova usa dois clientes multiplayer e dois endpoints de controle TCP, com ControlStream de produção na frente do jogo sintético. Exige que cada lado receba o quadro do outro com 512 ossos e 16 efeitos. Falhou com o código da prévia 12 e passou com a correção. O teste de limite usa 65.535 caracteres exatos para ambos os comandos, recusa 65.536 e verifica as três formas de quebra/injeção. São testes de transporte/integração, sem execução do renderer.

Mantidos os testes Go/race/vet e a compilação Windows; CI também recompila o pacote completo e testa o instalador. SHA256/CRC, identidade de versão, hashes nativos e exports do ZIP são conferidos antes da entrega. Não houve mudança em código nativo, sites, hooks, inputs do gerador ou política de overlay. ABI 9/protocolo 9/gamelib ABI 4. Uma biblioteca válida da prévia 12 já reconstruída é reutilizável; não editar .id.

Este hotfix não altera o escopo da prévia 12, não confirma visualmente cabelo/chapéu e o rastro do spin attack e não entrega mundo/combate/IA completos. Consulte também a [auditoria da prévia 12](AUDITORIA_0.5.0-preview.12.md). A confirmação de gameplay posterior continua necessária.
