# Histórico do HyruleM

## 0.5.0-preview.8 (experimental)

- Ambiente do anfitrião com sucessão e restauração; seis peças de equipamento apenas visual.
- Frame nativo atômico, slots AS, deduplicação de locomoção e interpolação/suavização.
- Prefabs derivados localmente e diagnóstico de falhas por peça; ABI 5/protocolo 5.
- Presença/movimento da prévia 7 confirmados pelo jogador; novos visuais aguardam gameplay.
- [Instalação e limites](native/README.md).

## Em desenvolvimento — geração nativa 1

- Diagnóstico `gen1-diag.2`: corrige o caminho do log de processo do launcher, respeita configuração personalizada e separação de clientes; relatórios com data/hora e botão `Diagnosticar.cmd`.
- Análise estática do módulo 1.9.0 enviado: mapa de 14 funções, fingerprints e cinco relações de chamada verificadas. Inspeção somente de leitura confirma o código carregado antes de acessar objetos/vida do Link e contagem de atores. Isso não habilita criação de atores.

- Servidor separado, protocolo 2, dois jogadores, IP/porta configuráveis, estado de vida/inventário individual e recuperação da sessão em memória por dez minutos.
- Coleta exclusiva, equipamento/descarte próprios, ataque corpo a corpo com alcance/cooldown e durabilidade individual no servidor.
- Criatura seleciona o jogador mais próximo em alcance; transferência de autoridade invalida mensagens antigas e preserva o cooldown.
- Interface validada para futuro backend na thread do jogo; diagnóstico Windows de somente leitura e artefato de desenvolvimento separado no Actions.
- Testes concorrentes, transporte TCP real, privacidade entre destinatários, mundos distintos e negociação de capacidades.
- **Pendente:** implementar e validar criação de Link, animação, colisão, bolsa/vida e IA no runtime BOTW. Não é release jogável com atores nativos. A versão estável segue em 0.4.2.

[Estado, diagnóstico e limitações Gen1](docs/gen1/README.md).

## 0.4.2 — 13/11/2026 · hotfix

### Correções

- Issue #1: o leitor EVFL rejeitava fluxos válidos com zero entradas. Esses fluxos agora são aceitos, mantendo a validação de ponteiros, assinatura e contagens. O filtro não cria um substituto sem entradas para um evento já vazio.
- Issue #2: o multiplayer exigia `filter.json` pronto, mesmo quando a preparação anterior havia falhado. Agora confere/prepara sandbox e filtro antes de conectar; dois clientes locais compartilham uma fila de preparação cancelável.
- Um filtro ausente ou inválido não é reconstruído enquanto um jogo o utiliza; é necessário fechar os jogos nesse caso. Um filtro publicado válido pode ser reutilizado pelo segundo cliente.
- Erros de leitura EVFL passam a identificar qual parte da tabela falhou.

### Validação

- Fixtures EVFL V3 sintéticas geradas e lidas por uma biblioteca independente; o caso vazio reproduzia a falha antes da correção.
- Filtro completo com missão principal de fluxo vazio, preservação dos arquivos originais e de missões secundárias.
- Preparação a partir de uma pasta sem `filter.json`, aplicação do overlay, cache, dois clientes simultâneos, cancelamento e proteção de um jogo em execução.
- Sem arquivos Nintendo nos novos testes ou no pacote. A confirmação no jogo real continua necessária.

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
