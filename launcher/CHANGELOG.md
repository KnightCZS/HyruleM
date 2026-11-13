# 0.4.0 — multiplayer visual experimental

Menu nome/IP/porta; servidor TCP para dois jogadores; cliente 1 e 2 no mesmo launcher com saves/dados/controle isolados; ponte de telemetria e avatar procedural projetado sobre cada janela. Não é ator nativo de BOTW. Veja `../docs/MULTIPLAYER.md` e `../docs/VALIDACAO_0.4.md`.

# Alterações

## 0.3.0

- Filtro local de QuestProduct: remove Main e preserva Mini/Sub por padrão.
- Substituições de eventos narrativos com fluxo válido de um quadro e descritor próprio.
- Tratamento de filmes, timelines, pontos de entrada internos e pacotes residentes.
- Remoção de EventTags de campanha e limpeza de ligações para tags removidas.
- Atualização da RSTB para os recursos gerados.
- Política editável, cache, relatório e atualização do filtro sem resetar o save.
- Modo de compatibilidade para jogar o mesmo sandbox sem a camada.
- Torres e runas deixam de receber qualquer tentativa de desbloqueio automático.
- Conflitos reais de opções de lançamento são relatados antes de iniciar.
- Novos testes de formatos binários, preservação de secundárias e integração.
- Filtro ainda pendente de validação dentro do BOTW real.

## 0.2.0

- Sandbox experimental com save separado da campanha.
- Cópia do slot normal mais recente do perfil ativo ou seleção manual de base Switch.
- Seis pontos de início e preparação do paraglider e flags conhecidas das runas.
- Retomada do progresso do sandbox sem repetir o reset inicial.
- Novo sandbox com backup automático do anterior e publicação após preparo completo.
- Ferramentas locais de posição, teleporte, horário, clima, voo, rupias e kit de itens.
- Verificação de jogador carregado e acompanhamento do estado de teleporte.
- Limpeza das variáveis de overlay e testes ao iniciar campanha.
- Diagnóstico inclui logs próprios do sandbox.
- Testes de saves, preservação, falhas, perfis e protocolo TCP.
- Campanha ainda presente; multiplayer ainda não implementado.

## 0.1.0

- Launcher Windows, configuração, instalação verificada da recomp, preparação externa e início offline.
