# Histórico do HyruleM

## 0.4.1 — 13/11/2026

### Adições

- Seleção da interface de hospedagem entre os IPv4 do computador, incluindo adaptadores VPN/Radmin, com porta TCP escolhida pelo usuário.
- Botão Atualizar IPs e indicação do endereço para convidar outro jogador.
- Reunir em Hateno: nova tentativa de teleporte sem abandonar a sessão de rede.
- Cancelar preparação e diagnóstico consolidado dos dois clientes, processos e servidor.
- Publicação de ZIP e SHA256 em Releases, depois dos testes Linux/Windows, regressão da interface nativa e compilação.

### Alterações

- A preparação dos arquivos acontece antes da conexão, e o início do processo ocorre fora do thread da interface.
- A consulta de sessão ocupada não espera varredura de processos, acesso a disco ou CreateProcess.
- Carregamento do jogo e falha de teleporte mantêm a presença de rede; a posição atual pode ser transmitida após falha de posicionamento.
- Barra lateral fixa, status sempre visível, menu multiplayer com rolagem e fonte própria por DPI.

### Correções

- Erros da preparação deixavam de aparecer quando um fechamento normal de socket os sobrescrevia com “Desconectado”.
- Uma falha no posicionamento inicial encerrava a ponte e a conexão inteira.
- Milhares de atualizações de progresso geravam milhares de mensagens nativas e redesenhos completos; agora o progresso é agregado e apenas o último valor é exibido.
- Paint intermediário, limpeza de fundo e reposicionamento com redesenho de cada controle causavam flashes e resíduos ao mover/rolar/redimensionar. A pintura principal agora usa buffer.
- Erros de início rápido do processo não devem ser sobrescritos por confirmação tardia de abertura.
- Reiniciar a hospedagem aguarda a porta anterior ser liberada; interface e porta ficam protegidas de edição enquanto o servidor está ativo.

### Limites da validação

Os testes usam TCP real, processos/controladores sintéticos e Win32 real na CI. Não há arquivos de BOTW nem uma rede Radmin instalada nesse ambiente. A execução de duas sessões reais do jogo, o comportamento na máquina do usuário e a calibração do avatar continuam sujeitos ao teste manual. O remoto ainda é um avatar externo, sem ator nativo, colisão ou oclusão.

## 0.4.0 — 13/11/2026

### Adições

- Primeiro servidor TCP para duas pessoas, nomes, presença e transmissão de poses.
- Dois clientes do Savage Wind com saves, dados e portas de controle separados.
- Ponte de posição/câmera e avatar procedural projetado sobre cada janela.

### Problemas relatados

Desconexão imediata, travamento aparente ao iniciar sandbox e problemas visuais na interface. Consulte 0.4.1 para as correções e a separação entre testes automatizados e teste no jogo.
