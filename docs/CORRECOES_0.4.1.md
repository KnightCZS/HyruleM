# Análise e validação das correções 0.4.1

## Problemas identificados no código

| Sintoma | Problema encontrado | Correção |
| --- | --- | --- |
| “Desconectado” imediatamente, sem explicação | O cliente conectava antes de validar os dados locais; quando a preparação/início falhava, fechar o socket podia sobrescrever a mensagem original | Preparar antes de conectar e preservar o erro da preparação ou do processo |
| Falha de posicionamento derruba a sessão | A ponte encerrava sua rotina e fechava a conexão após um erro de teleporte | Manter a rede, transmitir a posição atual e oferecer nova tentativa por Reunir em Hateno |
| Interface deixa de responder ao iniciar | O início do processo ocorria no thread da interface; a consulta Busy também podia esperar um mutex mantido durante varredura de processos e operações de arquivo | Início assíncrono e reserva curta do estado da sessão, sem manter o mutex durante I/O |
| Preparação parece congelada | Cada arquivo gerava notificações nativas e atualização completa de controles, mesmo quando várias atualizações seriam descartadas visualmente | Agregar progresso e acordar a interface uma vez por lote, sem refazer o layout para cada atualização |
| Flashes/resíduos ao mexer na janela | Pintura direta e movimentos de controles com redesenho individual expunham estados intermediários | Buffer de pintura, reposicionamento sem redesenho individual, barra lateral fixa e status visível |
| Difícil configurar LAN/Radmin | O servidor aceitava todas as interfaces, mas o menu só indicava loopback e não oferecia escolha do adaptador | Listar IPv4 do computador/VPN, selecionar interface e porta, indicar endereço de conexão |

Sem os logs da execução original, não é possível atribuir todos os sintomas da máquina do usuário a uma única causa. As falhas acima foram encontradas no código e receberam testes de regressão.

## Executado

No commit de implementação `71a4e2539375dca55f52c5dc66cbbf627907f3a5`:

- Testes Linux com detector de corrida.
- Testes de dados, saves, junctions e rede em Windows Server 2025.
- Conexão mantida por mais de oito segundos sem mundo carregado; falha de teleporte não desconecta; nova tentativa funciona.
- Hospedagem e conexão TCP pelo adaptador local, com porta definida; endereços VPN não são descartados por não serem privados.
- Teste Win32 de 20.000 atualizações de progresso, 120 operações de rolagem/layout, contagem de recursos GDI e preparação/início de sandbox sintético.
- Erro de preparação multiplayer preservado e ausência de conexão quando os dados locais são inválidos.
- Compilação dos dois executáveis e abertura do launcher/menu.
- ZIP e SHA256 publicados na release `v0.4.1`.

A etapa de publicação também tentou registrar a versão histórica 0.4.0; o GitHub recusou essa operação para o commit antigo. A release 0.4.1 e seus arquivos foram publicados antes disso. A automação foi corrigida para publicar somente a versão atual; o histórico 0.4.0 permanece no changelog e em `releases/v0.4.0.md`.

## Ainda exige teste no jogo

Duas sessões reais de BOTW, conexão por uma rede Radmin instalada, compatibilidade do runtime com a máquina do usuário e alinhamento visual do avatar. Nenhum arquivo Nintendo foi utilizado na CI. O avatar remoto continua externo e não possui colisão/oclusão.

Para atualizar, extraia o ZIP novo em outra pasta e use o novo executável. Não recrie saves para instalar a atualização. Se houver falha, o diagnóstico do menu inclui os dois clientes e o servidor, preservando agora o motivo original.
